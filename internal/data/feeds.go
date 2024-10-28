package data

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/Blue-Davinci/OptiVest/internal/database"
	"github.com/Blue-Davinci/OptiVest/internal/validator"
	"github.com/araddon/dateparse"
)

type FeedManagerModel struct {
	DB *database.Queries
}

const (
	DefaultFeedManDBContextTimeout = 5 * time.Second
	DefaultImageURL                = "https://images.unsplash.com/photo-1542396601-dca920ea2807?q=80&w=1351&auto=format&fit=crop&ixlib=rb-4.0.3&ixid=M3wxMjA3fDB8MHxwaG90by1wYWdlfHx8fGVufDB8fHx8fA%3D%3D"
)
const (
	// Feed types
	FeedManFeedTypeRSS = database.FeedTypeRss
	FeedManFeedTypeAPI = database.FeedTypeJson
	// Approval statuses
	FeedManApprovalStatusPending  = database.FeedApprovalStatusPending
	FeedManApprovalStatusApproved = database.FeedApprovalStatusApproved
	FeedManApprovalStatusRejected = database.FeedApprovalStatusRejected
)

var (
	ErrDuplicateFeed          = errors.New("feed with this URL already exists")
	ErrInvalidFeedType        = errors.New("invalid feed type")
	ErrInvalidApprovalStatus  = errors.New("invalid approval status")
	ErrContextDeadline        = errors.New("context deadline exceeded")
	ErrUnableToDetectFeedType = errors.New("unable to detect the feed type in the url")
	ErrDuplicateFavorite      = errors.New("favorite already exists")
)

// feed struct
type Feed struct {
	ID              int64                       `json:"id"`
	UserID          int64                       `json:"user_id"`
	Name            string                      `json:"name"`
	URL             string                      `json:"url"`
	ImgUrl          string                      `json:"img_url"`
	FeedType        database.FeedType           `json:"feed_type"`
	FeedCategory    string                      `json:"feed_category"`
	FeedDescription string                      `json:"feed_description"`
	IsHidden        bool                        `json:"is_hidden"`
	ApprovalStatus  database.FeedApprovalStatus `json:"approval_status"`
	Version         int32                       `json:"version"`
	CreatedAt       time.Time                   `json:"created_at"`
	UpdatedAt       time.Time                   `json:"updated_at"`
	LastFetchedAt   time.Time                   `json:"last_fetched_at"`
}

// We make a solo struct that will hold a returned Post Favorite
type RSSPostFavorite struct {
	ID         int64     `json:"id"`
	PostID     int64     `json:"post_id"`
	FeedID     int64     `json:"feed_id"`
	UserID     int64     `json:"-"`
	Created_At time.Time `json:"created_at"`
}

// PostFeedWithFavoriteTag returns an RSSFeed with a favorite tag
type RSSPostWithFavoriteTag struct {
	FeedID      int64    `json:"feed_id"`
	IsFavorited bool     `json:"is_favorited"`
	RSSFeed     *RSSFeed `json:"rss_feed"`
}

// RSSFeed is a struct that represents what our RSS Feed looks like
type RSSFeed struct {
	ID        int64     `json:"id"`
	Createdat time.Time `json:"created_at"`
	Updatedat time.Time `json:"updated_at"`
	Feed_ID   int64     `json:"feed_id"`
	Channel   struct {
		Title       string    `xml:"title"`
		Link        string    `xml:"link"`
		Description string    `xml:"description"`
		Language    string    `xml:"language"`
		Item        []RSSItem `xml:"item"`
	} `xml:"channel"`
	RetryMax   int32 `json:"-"`
	StatusCode int32 `json:"-"`
}

type RSSItem struct {
	Title       string `xml:"title"`
	Link        string `xml:"link"`
	Description string `xml:"description"`
	Content     string `xml:"content"`
	PubDate     string `xml:"pubDate"`
	ImageURL    string `xml:"image_url"`
}

func ValidateFeed(v *validator.Validator, feed *Feed) {
	ValidateName(v, feed.Name, "name")
	ValidateName(v, feed.URL, "url")
	ValidateName(v, string(feed.FeedType), "feed_type")
	ValidateName(v, feed.FeedCategory, "feed_category")
	// validate is hidden boolean
	ValidateBoolean(v, feed.IsHidden, "is_hidden")

}
func ValidateRSSPostFavorite(v *validator.Validator, rssPostFavorite *RSSPostFavorite) {
	ValidateURLID(v, rssPostFavorite.PostID, "post_id")
	ValidateURLID(v, rssPostFavorite.FeedID, "feed_id")
}

// MapFeedApprovalStatusToConstant() is a helper function that maps a string to a FeedApprovalStatus constant
func (m FeedManagerModel) MapFeedApprovalStatusToConstant(status string) (database.FeedApprovalStatus, error) {
	switch status {
	case "pending":
		return FeedManApprovalStatusPending, nil
	case "approved":
		return FeedManApprovalStatusApproved, nil
	case "rejected":
		return FeedManApprovalStatusRejected, nil
	default:
		return "", ErrInvalidApprovalStatus
	}
}

// MapFeedTypeToConstant() is a helper function that maps a string to a FeedType constant
func (m FeedManagerModel) MapFeedTypeToConstant(feedType string) (database.FeedType, error) {
	switch feedType {
	case "rss":
		return FeedManFeedTypeRSS, nil
	case "api":
		return FeedManFeedTypeAPI, nil
	default:
		return "", ErrInvalidFeedType
	}
}

// CreateNewFeed() is a method that creates a new feed
// Feeds will be used to get news information that will be displayed to the user
// We will take in a *feed, enrich with new data and return an error.
func (m FeedManagerModel) CreateNewFeed(userID int64, feed *Feed) error {
	ctx, cancel := contextGenerator(context.Background(), DefaultFinManDBContextTimeout)
	defer cancel()
	// insert
	feedInfo, err := m.DB.CreateNewFeed(ctx, database.CreateNewFeedParams{
		UserID:          userID,
		Name:            feed.Name,
		Url:             feed.URL,
		ImgUrl:          sql.NullString{String: feed.ImgUrl, Valid: true},
		FeedType:        feed.FeedType,
		FeedCategory:    feed.FeedCategory,
		FeedDescription: sql.NullString{String: feed.FeedDescription, Valid: true},
		IsHidden:        feed.IsHidden,
	})
	if err != nil {
		switch {
		case err.Error() == `pq: duplicate key value violates unique constraint "feeds_url_key"`:
			return ErrDuplicateFeed
		default:
			return err
		}
	}
	// update feed with new data
	feed.ID = feedInfo.ID
	feed.UserID = userID
	feed.CreatedAt = feedInfo.CreatedAt
	feed.UpdatedAt = feedInfo.UpdatedAt
	feed.Version = feedInfo.Version
	feed.ApprovalStatus = feedInfo.ApprovalStatus
	// done
	return nil
}

// UpdateFeed() is a method that will Update an existing Feed
// We recieve a userID and *feed, and use that to update the feed
func (m FeedManagerModel) UpdateFeed(userID int64, feed *Feed) error {
	ctx, cancel := contextGenerator(context.Background(), DefaultFinManDBContextTimeout)
	defer cancel()
	// update
	updatedInfo, err := m.DB.UpdateFeed(ctx, database.UpdateFeedParams{
		ID:              feed.ID,
		UserID:          userID,
		Name:            feed.Name,
		Url:             feed.URL,
		ImgUrl:          sql.NullString{String: feed.ImgUrl, Valid: true},
		FeedType:        feed.FeedType,
		FeedCategory:    feed.FeedCategory,
		FeedDescription: sql.NullString{String: feed.FeedDescription, Valid: true},
		ApprovalStatus:  feed.ApprovalStatus,
		IsHidden:        feed.IsHidden,
		Version:         feed.Version,
	})
	if err != nil {
		switch {
		case errors.Is(err, sql.ErrNoRows):
			return ErrEditConflict
		default:
			return err
		}
	}
	// update feed with new data
	feed.UpdatedAt = updatedInfo.UpdatedAt
	feed.Version = updatedInfo.Version
	// done
	return nil
}

//	DeleteFeedByID() is a method that will delete a feed by its ID
//
// We will recieve a feedID and return a feedID and an error
func (m FeedManagerModel) DeleteFeedByID(userID, feedID int64) (*int64, error) {
	ctx, cancel := contextGenerator(context.Background(), DefaultFinManDBContextTimeout)
	defer cancel()
	// delete
	feedID, err := m.DB.DeleteFeedByID(ctx, database.DeleteFeedByIDParams{
		ID:     feedID,
		UserID: userID,
	})
	if err != nil {
		switch {
		case errors.Is(err, sql.ErrNoRows):
			return nil, ErrGeneralRecordNotFound
		default:
			return nil, err
		}
	}
	// done
	return &feedID, nil
}

// GetFeedByID() is a method that will return a feed by its ID
// We will recieve a feedID and return a *feed and an error
func (m FeedManagerModel) GetFeedByID(feedID int64) (*Feed, error) {
	ctx, cancel := contextGenerator(context.Background(), DefaultFinManDBContextTimeout)
	defer cancel()
	// get feed
	feedRow, err := m.DB.GetFeedByID(ctx, feedID)
	if err != nil {
		switch {
		case errors.Is(err, sql.ErrNoRows):
			return nil, ErrGeneralRecordNotFound
		default:
			return nil, err
		}
	}
	// populate feed
	feed := populateFeed(feedRow)
	// done
	return feed, nil
}

// GetNextFeedsToFetch() is a method that will return the next feeds to fetch
// We will recieve a limit and return a slice of *feed and an error
func (m FeedManagerModel) GetNextFeedsToFetch(limit int32) ([]*Feed, error) {
	fmt.Println("Getting next feeds to fetch: ", limit)
	ctx, cancel := contextGenerator(context.Background(), DefaultFinManDBContextTimeout)
	defer cancel()
	// get feeds
	feedRows, err := m.DB.GetNextFeedsToFetch(ctx, limit)
	if err != nil {
		return nil, err
	}
	// populate feeds
	feeds := []*Feed{}
	for _, feedRow := range feedRows {
		feed := populateFeed(feedRow)
		//fmt.Println("Feed Found: ", feed.Name)
		feeds = append(feeds, feed)
	}

	// done
	return feeds, nil
}

// MarkFeedAsFetched() is a method that will mark a feed as fetched
// We return an error if there is one
func (m FeedManagerModel) MarkFeedAsFetched(feedID int64) error {
	ctx, cancel := contextGenerator(context.Background(), DefaultFinManDBContextTimeout)
	defer cancel()
	// mark feed as fetched
	_, err := m.DB.MarkFeedAsFetched(ctx, feedID)
	if err != nil {
		return err
	}
	// done
	return nil
}

// ==============================================================================================
// Posts
// ==============================================================================================
func (m FeedManagerModel) CreateRssFeedPost(rssFeed *RSSFeed, feedID int64) error {
	// Get channel Info
	ChannelTitle := rssFeed.Channel.Title
	ChannelUrl := rssFeed.Channel.Link
	ChannelDescription := rssFeed.Channel.Description
	ChannelLanguage := rssFeed.Channel.Language
	for _, item := range rssFeed.Channel.Item {
		// We use dateparse to parse a variety of possible date/time data rather than using
		// the time.Parse() function which is more strict.
		// We use ParseAny()
		publishedAt, err := dateparse.ParseAny(item.PubDate)
		if err != nil {
			continue
		}
		_, err = m.DB.CreateRssFeedPost(context.Background(), database.CreateRssFeedPostParams{
			// Channel info
			Channeltitle:       ChannelTitle,
			Channelurl:         sql.NullString{String: ChannelUrl, Valid: ChannelUrl != ""},
			Channeldescription: sql.NullString{String: ChannelDescription, Valid: ChannelDescription != ""},
			Channellanguage:    sql.NullString{String: ChannelLanguage, Valid: ChannelLanguage != ""},
			// Item Info
			Itemtitle:       item.Title,
			Itemdescription: sql.NullString{String: item.Description, Valid: rssFeed.Channel.Description != ""},
			Itemcontent:     sql.NullString{String: item.Content, Valid: item.Content != ""},
			ItempublishedAt: publishedAt,
			Itemurl:         item.Link,
			ImgUrl:          item.ImageURL,
			FeedID:          feedID,
		})
		// Our db should not contain the same  URL/Post twice, so we just ignore this error (is it an error really?)
		// and actually print real ones.
		if err != nil && err.Error() != `pq: duplicate key value violates unique constraint "rssfeed_posts_itemurl_key"` {
			fmt.Println("Couldn't create post for: ", item.Title, "Error: ", err)
		}
	}
	return nil
}

// CreateNewFavoriteOnPost() is a method that will create a new favorite on a post
// We acept a new post favorite and return an id, createdat and an error
func (m FeedManagerModel) CreateNewFavoriteOnPost(userID int64, rssFavoritePost *RSSPostFavorite) error {
	ctx, cancel := contextGenerator(context.Background(), DefaultFinManDBContextTimeout)
	defer cancel()
	// create
	favoriteInfo, err := m.DB.CreateNewFavoriteOnPost(ctx, database.CreateNewFavoriteOnPostParams{
		PostID: rssFavoritePost.PostID,
		FeedID: rssFavoritePost.FeedID,
		UserID: userID,
	})
	if err != nil {
		switch {
		case err.Error() != `pq: duplicate key value violates unique constraint "favorite_posts_post_id_key"`:
			return ErrDuplicateFavorite
		default:
			return err
		}
	}
	// update post favorite with new data
	rssFavoritePost.ID = favoriteInfo.ID
	rssFavoritePost.Created_At = favoriteInfo.CreatedAt
	// done
	return nil
}

// DeleteFavoriteOnPost() is a method that will delete a favorite on a post
// We will recieve a userID, postID and return a postID and an error
func (m FeedManagerModel) DeleteFavoriteOnPost(userID, postID int64) (*int64, error) {
	ctx, cancel := contextGenerator(context.Background(), DefaultFinManDBContextTimeout)
	defer cancel()
	// delete
	postID, err := m.DB.DeleteFavoriteOnPost(ctx, database.DeleteFavoriteOnPostParams{
		UserID: userID,
		PostID: postID,
	})
	if err != nil {
		switch {
		case errors.Is(err, sql.ErrNoRows):
			return nil, ErrGeneralRecordNotFound
		default:
			return nil, err
		}
	}
	// done
	return &postID, nil
}

// GetRssFeedPostByID() is a method that will return a post by its ID
// We will recieve a postID and return a *RSSPostFavorite and an error
func (m FeedManagerModel) GetRssFeedPostByID(postID int64) (*RSSFeed, error) {
	ctx, cancel := contextGenerator(context.Background(), DefaultFinManDBContextTimeout)
	defer cancel()
	// get post
	postRow, err := m.DB.GetRssFeedPostByID(ctx, postID)
	if err != nil {
		switch {
		case errors.Is(err, sql.ErrNoRows):
			return nil, ErrGeneralRecordNotFound
		default:
			return nil, err
		}
	}
	// populate post
	post := populateRssFeedPost(postRow)
	// done
	return post, nil
}

// GetAllRSSPostWithFavoriteTag() is a method that will return all posts with a favorite tag
// This will return a slice of *RSSPostWithFavoriteTag, a metadata struct and an error
// It supports both search and pagination
func (m FeedManagerModel) GetAllRSSPostWithFavoriteTag(userID, feedID int64, itemName, postCategory string, filters Filters) ([]*RSSPostWithFavoriteTag, Metadata, error) {
	ctx, cancel := contextGenerator(context.Background(), DefaultFinManDBContextTimeout)
	defer cancel()
	// get all posts
	postRows, err := m.DB.GetAllRSSPostWithFavoriteTag(ctx, database.GetAllRSSPostWithFavoriteTagParams{
		UserID:  userID,
		Column2: itemName,
		Column3: feedID,
		Limit:   int32(filters.limit()),
		Offset:  int32(filters.offset()),
		Column6: postCategory,
	})
	if err != nil {
		return nil, Metadata{}, err
	}
	//  check if there are no posts
	if len(postRows) == 0 {
		return nil, Metadata{}, ErrGeneralRecordNotFound
	}
	// populate posts
	posts := []*RSSPostWithFavoriteTag{}
	totalFeeds := 0
	for _, postRow := range postRows {
		totalFeeds = int(postRow.TotalCount)
		post := populateRSSPostWithFavoriteTag(postRow)
		posts = append(posts, post)
	}
	// make metadata struct
	metadata := calculateMetadata(totalFeeds, filters.Page, filters.PageSize)
	// done
	return posts, metadata, nil
}

// populateRSSPostWithFavoriteTag() is a helper function that will populate a post with a favorite tag
// will return a *RSSPostWithFavoriteTag which is a struct that contains a RSSFeed and a boolean
// We can use populateRSSFeedPost() for the RSSFeed and just add the boolean
func populateRSSPostWithFavoriteTag(postRow interface{}) *RSSPostWithFavoriteTag {
	switch postRow := postRow.(type) {
	case database.GetAllRSSPostWithFavoriteTagRow:
		return &RSSPostWithFavoriteTag{
			FeedID:      postRow.FeedID,
			IsFavorited: postRow.IsFavorite,
			RSSFeed:     populateRssFeedPost(postRow),
		}
	default:
		return nil
	}
}

// populateRssFeedPost() is a helper function that will populate a post
func populateRssFeedPost(postRow interface{}) *RSSFeed {
	switch postRow := postRow.(type) {
	case database.GetRssFeedPostByIDRow:
		return &RSSFeed{
			ID:        postRow.ID,
			Createdat: postRow.CreatedAt,
			Updatedat: postRow.UpdatedAt,
			Feed_ID:   postRow.FeedID,
			Channel: struct {
				Title       string    `xml:"title"`
				Link        string    `xml:"link"`
				Description string    `xml:"description"`
				Language    string    `xml:"language"`
				Item        []RSSItem `xml:"item"`
			}{
				Title:       postRow.Channeltitle,
				Link:        postRow.Channelurl.String,
				Description: postRow.Channeldescription.String,
				Language:    postRow.Channellanguage.String,
				Item: []RSSItem{
					{
						Title:       postRow.Itemtitle,
						Description: postRow.Itemdescription.String,
						Content:     postRow.Itemcontent.String,
						PubDate:     postRow.ItempublishedAt.Format(time.RFC1123),
						Link:        postRow.Itemurl,
						ImageURL:    postRow.ImgUrl,
					},
				},
			},
		}
	case database.GetAllRSSPostWithFavoriteTagRow:
		return &RSSFeed{
			ID:        postRow.ID,
			Createdat: postRow.CreatedAt,
			Updatedat: postRow.UpdatedAt,
			Feed_ID:   postRow.FeedID,
			Channel: struct {
				Title       string    `xml:"title"`
				Link        string    `xml:"link"`
				Description string    `xml:"description"`
				Language    string    `xml:"language"`
				Item        []RSSItem `xml:"item"`
			}{
				Title:       postRow.Channeltitle,
				Link:        postRow.Channelurl.String,
				Description: postRow.Channeldescription.String,
				Language:    postRow.Channellanguage.String,
				Item: []RSSItem{
					{
						Title:       postRow.Itemtitle,
						Description: postRow.Itemdescription.String,
						Content:     postRow.Itemcontent.String,
						PubDate:     postRow.ItempublishedAt.Format(time.RFC1123),
						Link:        postRow.Itemurl,
						ImageURL:    postRow.ImgUrl,
					},
				},
			},
		}

	default:
		return nil
	}
}

// populateFeed() is a helper function that will populate a feed
func populateFeed(feedRow interface{}) *Feed {
	switch feed := feedRow.(type) {
	case database.Feed:
		return &Feed{
			ID:              feed.ID,
			UserID:          feed.UserID,
			Name:            feed.Name,
			URL:             feed.Url,
			ImgUrl:          feed.ImgUrl.String,
			FeedType:        feed.FeedType,
			FeedCategory:    feed.FeedCategory,
			FeedDescription: feed.FeedDescription.String,
			IsHidden:        feed.IsHidden,
			ApprovalStatus:  feed.ApprovalStatus,
			Version:         feed.Version,
			CreatedAt:       feed.CreatedAt,
			UpdatedAt:       feed.UpdatedAt,
		}
	case database.GetNextFeedsToFetchRow:
		return &Feed{
			ID:              feed.ID,
			UserID:          feed.UserID,
			Name:            feed.Name,
			URL:             feed.Url,
			ImgUrl:          feed.ImgUrl.String,
			FeedType:        feed.FeedType,
			FeedCategory:    feed.FeedCategory,
			FeedDescription: feed.FeedDescription.String,
			IsHidden:        feed.IsHidden,
			ApprovalStatus:  feed.ApprovalStatus,
			Version:         feed.Version,
			CreatedAt:       feed.CreatedAt,
			UpdatedAt:       feed.UpdatedAt,
		}
	default:
		return nil
	}
}
