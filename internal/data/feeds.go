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

// CreateNewFeed creates a new feed. Feeds are used to surface news
// information to the user. We take a *Feed, enrich it with new data, and
// return an error. ctx flows from the originating HTTP request.
func (m FeedManagerModel) CreateNewFeed(ctx context.Context, userID int64, feed *Feed) error {
	ctx, cancel := contextGenerator(ctx, DefaultFinManDBContextTimeout)
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

// UpdateFeed updates an existing feed. We receive a userID and *Feed
// and use that to perform the update. ctx flows from the originating
// HTTP request.
func (m FeedManagerModel) UpdateFeed(ctx context.Context, userID int64, feed *Feed) error {
	ctx, cancel := contextGenerator(ctx, DefaultFinManDBContextTimeout)
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

// DeleteFeedByID deletes a feed by its ID. ctx flows from the
// originating HTTP request.
func (m FeedManagerModel) DeleteFeedByID(ctx context.Context, userID, feedID int64) (*int64, error) {
	ctx, cancel := contextGenerator(ctx, DefaultFinManDBContextTimeout)
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

// GetFeedByID returns a feed by its ID. ctx flows from the originating
// HTTP request.
func (m FeedManagerModel) GetFeedByID(ctx context.Context, feedID int64) (*Feed, error) {
	ctx, cancel := contextGenerator(ctx, DefaultFinManDBContextTimeout)
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

// GetNextFeedsToFetch returns the next feeds to fetch. We receive a limit
// and return a slice of *Feed. ctx flows from the caller (the RSS scraper
// goroutine, which derives from app.ctx).
func (m FeedManagerModel) GetNextFeedsToFetch(ctx context.Context, limit int32) ([]*Feed, error) {
	fmt.Println("Getting next feeds to fetch: ", limit)
	ctx, cancel := contextGenerator(ctx, DefaultFinManDBContextTimeout)
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

// MarkFeedAsFetched marks a feed as fetched. ctx flows from the caller
// (the RSS scraper goroutine, which derives from app.ctx).
func (m FeedManagerModel) MarkFeedAsFetched(ctx context.Context, feedID int64) error {
	ctx, cancel := contextGenerator(ctx, DefaultFinManDBContextTimeout)
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
// CreateRssFeedPost inserts the parsed items from a fetched RSS feed.
// ctx flows from the RSS scraper goroutine (derived from app.ctx) so the
// in-flight INSERTs are cancelled cleanly on graceful shutdown.
func (m FeedManagerModel) CreateRssFeedPost(ctx context.Context, rssFeed *RSSFeed, feedID int64) error {
	ChannelTitle := rssFeed.Channel.Title
	ChannelUrl := rssFeed.Channel.Link
	ChannelDescription := rssFeed.Channel.Description
	ChannelLanguage := rssFeed.Channel.Language
	for _, item := range rssFeed.Channel.Item {
		// We use dateparse to parse a variety of possible date/time
		// formats rather than time.Parse() which is more strict.
		publishedAt, err := dateparse.ParseAny(item.PubDate)
		if err != nil {
			continue
		}
		_, err = m.DB.CreateRssFeedPost(ctx, database.CreateRssFeedPostParams{
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

// CreateNewFavoriteOnPost creates a new favorite on a post. ctx flows
// from the originating HTTP request.
func (m FeedManagerModel) CreateNewFavoriteOnPost(ctx context.Context, userID int64, rssFavoritePost *RSSPostFavorite) error {
	ctx, cancel := contextGenerator(ctx, DefaultFinManDBContextTimeout)
	defer cancel()
	// create
	favoriteInfo, err := m.DB.CreateNewFavoriteOnPost(ctx, database.CreateNewFavoriteOnPostParams{
		PostID: rssFavoritePost.PostID,
		FeedID: rssFavoritePost.FeedID,
		UserID: userID,
	})
	if err != nil {
		switch {
		case err.Error() == `pq: duplicate key value violates unique constraint "favorite_posts_post_id_key"`:
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

// DeleteFavoriteOnPost deletes a favorite on a post. ctx flows from the
// originating HTTP request.
func (m FeedManagerModel) DeleteFavoriteOnPost(ctx context.Context, userID, postID int64) (*int64, error) {
	ctx, cancel := contextGenerator(ctx, DefaultFinManDBContextTimeout)
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

// GetRssFeedPostByID returns a post by its ID. ctx flows from the
// originating HTTP request.
func (m FeedManagerModel) GetRssFeedPostByID(ctx context.Context, postID int64) (*RSSFeed, error) {
	ctx, cancel := contextGenerator(ctx, DefaultFinManDBContextTimeout)
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

// GetAllRSSPostWithFavoriteTag returns all posts with a favorite tag.
// Returns a slice of *RSSPostWithFavoriteTag, a metadata struct, and an
// error. Supports both search and pagination. ctx flows from the
// originating HTTP request.
func (m FeedManagerModel) GetAllRSSPostWithFavoriteTag(ctx context.Context, userID, feedID int64, itemName, postCategory string, filters Filters) ([]*RSSPostWithFavoriteTag, Metadata, error) {
	ctx, cancel := contextGenerator(ctx, DefaultFinManDBContextTimeout)
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
