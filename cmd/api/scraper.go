package main

import (
	"bytes"
	"fmt"
	"io"
	"net/http"

	"github.com/Blue-Davinci/OptiVest/internal/data"
	"github.com/microcosm-cc/bluemonday"
	"github.com/mmcdole/gofeed"
	"github.com/mmcdole/gofeed/atom"
	"go.uber.org/zap"
)

// rssFeedScraper() is the main method which performs scraping for each
// individual feed. It takes in an indvidiual Feed, updates its last fetched
// using MarkFeedAsFetched() and then saved the data to our DB
func (app *application) rssFeedScraper(feed *data.Feed) {
	app.logger.Info("Fetching feed", zap.String("Feed Name", feed.Name), zap.Int64("Feed ID", feed.ID))
	// we want to fetch each of the feeds concurrently, so we make a wait group
	// using our app.background(func(){}) through a for loop to iterate over the feeds starting a routine for each feed
	app.background(func() {
		// get the feed data
		err := app.models.FeedManager.MarkFeedAsFetched(feed.ID)
		if err != nil {
			app.logger.Info("An error occurred while marking feed as fetched", zap.String("Feed Name", feed.Name), zap.Int64("Feed ID", feed.ID))
			return
		}
		// call our GetRSSFeeds to return all feeds for each specific URL
		rssFeeds, err := app.scraperGetRSSFeeds(
			app.config.scraper.scraperclient.retrymax,
			app.config.scraper.scraperclient.timeout,
			feed.URL, app.config.sanitization.sanitizer)
		if err != nil {
			switch {
			case err == data.ErrContextDeadline:
				// create a context deadline status code
				// ToDo:  create our error detail with a context errorType
			case err == data.ErrUnableToDetectFeedType:
				// ToDo: create our error detail with a feed errorType
			default:
				app.logger.Info("An error occurred while fetching feed", zap.String("Feed Name", feed.Name), zap.Int64("Feed ID", feed.ID), zap.Error(err))
			}
			if err == data.ErrContextDeadline {
				return
			}
		}
		// store the fetched data into our DB
		err = app.models.FeedManager.CreateRssFeedPost(rssFeeds, feed.ID)
		if err != nil {
			app.logger.Info("An error occurred while saving feed data", zap.String("Feed Name", feed.Name), zap.Int64("Feed ID", feed.ID), zap.String("Error", err.Error()))
			return
		}

		/*app.logger.PrintInfo("Finished collecting feeds for: ", map[string]string{
			"Name":   feed.Name,
			"Posts:": fmt.Sprintf("%d", len(rssFeeds.Channel.Item)),
		})*/
	})
}

// RssFeedDecoder() will decide which type of URL we are fetching i.e. Atom or RSS
// and then choose different decoders for each type of feed
func (app *application) RssFeedDecoderDecider(url string, rssFeed *data.RSSFeed, sanitizer *bluemonday.Policy, resp *http.Response) error {
	// Read the entire response body into a byte slice
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}

	// Attempt to parse using gofeed
	fp := gofeed.NewParser()
	feed, err := fp.Parse(bytes.NewReader(data))
	if err == nil && feed != nil {
		convertGofeedToRSSFeed(rssFeed, feed, sanitizer)
		return nil
	} else if err != nil {
		// Log or return specific error for gofeed parsing failure
		return fmt.Errorf("gofeed parsing error: %w", err)
	}

	// Attempt to parse using atom parser
	atomParser := &atom.Parser{}
	atomFeed, err := atomParser.Parse(bytes.NewReader(data))
	if err == nil && atomFeed != nil {
		convertAtomfeedToRSSFeed(rssFeed, atomFeed, sanitizer)
		return nil
	} else if err != nil {
		// Log or return specific error for atom parsing failure
		return fmt.Errorf("atom parsing error: %w", err)
	}

	// If all parsing attempts fail, return a generic error
	return fmt.Errorf("unable to parse feed from URL: %s", url)
}

// =======================================================================================================================
//
//	CONVERTORS
//
// =======================================================================================================================

// convertAtomfeedToRSSFeed() will convert an atom.Feed struct to our RSSFeed struct
// This is done by copying the fields from the atom.Feed struct to our RSSFeed struct
// acknowledging the differences in field items and field entries
func convertAtomfeedToRSSFeed(rssFeed *data.RSSFeed, feed *atom.Feed, sanitizer *bluemonday.Policy) {
	if rssFeed == nil || feed == nil {
		fmt.Println("RSSFeed pointer or atom.Feed pointer is nil")
		return
	}
	//proceed to fill the main channel fields
	rssFeed.Channel.Title = sanitizer.Sanitize(feed.Title)
	rssFeed.Channel.Description = sanitizer.Sanitize(feed.Subtitle)
	// Grab our first link as the main link for the channel
	if len(feed.Links) > 0 {
		rssFeed.Channel.Link = sanitizer.Sanitize(feed.Links[0].Href)
	}
	rssFeed.Channel.Language = sanitizer.Sanitize(feed.Language)
	// Use the correct field for Atom entries, which is `Entries` instead of `Items` as for RSS feeds
	rssFeed.Channel.Item = make([]data.RSSItem, len(feed.Entries)) // Allocate space for entries
	for i, entry := range feed.Entries {
		// As like RSS feeds, we use a default image URL if no image is found
		// We also use the link property to search for any image URLs
		imageURL := data.DefaultImageURL
		for _, link := range entry.Links {
			if link.Rel == "enclosure" || link.Type == "image/jpeg" || link.Type == "image/png" {
				imageURL = link.Href
				break // Found an image URL, exit the loop
			}
		}
		rssFeed.Channel.Item[i] = data.RSSItem{
			Title:       sanitizer.Sanitize(entry.Title),
			Link:        sanitizer.Sanitize(entry.Links[0].Href),
			Description: sanitizer.Sanitize(entry.Summary),
			Content:     sanitizer.Sanitize(entry.Content.Value),
			PubDate:     sanitizer.Sanitize(entry.Published),
			ImageURL:    imageURL, // sanitizer.Sanitize(imageURL)
		}
	}
}

// convertGofeedToRSSFeed() will convert a gofeed.Feed struct to our RSSFeed struct
// This is done by copying the fields from the gofeed.Feed struct to our RSSFeed struct
func convertGofeedToRSSFeed(rssFeed *data.RSSFeed, feed *gofeed.Feed, sanitizer *bluemonday.Policy) {
	if rssFeed == nil || feed == nil {
		fmt.Println("RSSFeed pointer or gofeed.Feed pointer is nil")
		return
	}
	// Fill the main channel fields
	rssFeed.Channel.Title = sanitizer.Sanitize(feed.Title)
	rssFeed.Channel.Link = sanitizer.Sanitize(feed.Link)
	rssFeed.Channel.Description = sanitizer.Sanitize(feed.Description)
	rssFeed.Channel.Language = sanitizer.Sanitize(feed.Language)
	// Use the correct field for RSS items
	rssFeed.Channel.Item = make([]data.RSSItem, len(feed.Items)) // Allocate space for items
	for i, item := range feed.Items {
		// As like Atom feeds, we use a default image URL if no image is found
		imageURL := data.DefaultImageURL
		if item.Image != nil {
			imageURL = item.Image.URL
		}
		/*
			// save dcontent to file
			fileName := fmt.Sprintf("item_%d.txt", i)
			filePath := filepath.Join("output", fileName)
			err := os.MkdirAll("output", os.ModePerm)
			if err != nil {
				fmt.Printf("Error creating directory: %v\n", err)
				continue
			}
			file, err := os.Create(filePath)
			if err != nil {
				fmt.Printf("Error creating file: %v\n", err)
				continue
			}
			defer file.Close()
			_, err = file.WriteString(item.Content)
			if err != nil {
				fmt.Printf("Error writing to file: %v\n", err)
				continue
			}
			/// -----------------
		*/
		rssFeed.Channel.Item[i] = data.RSSItem{
			Title:       sanitizer.Sanitize(item.Title),
			Link:        sanitizer.Sanitize(item.Link),
			Description: sanitizer.Sanitize(item.Description),
			Content:     sanitizer.Sanitize(item.Content),
			PubDate:     sanitizer.Sanitize(item.Published),
			ImageURL:    imageURL, // sanitizer.Sanitize(imageURL) Note: This breaks some images
		}
	}
}
