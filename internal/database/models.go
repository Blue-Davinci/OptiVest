// Code generated by sqlc. DO NOT EDIT.
// versions:
//   sqlc v1.26.0

package database

import (
	"database/sql"
	"database/sql/driver"
	"fmt"
	"time"

	"github.com/sqlc-dev/pqtype"
)

type FeedApprovalStatus string

const (
	FeedApprovalStatusPending  FeedApprovalStatus = "pending"
	FeedApprovalStatusApproved FeedApprovalStatus = "approved"
	FeedApprovalStatusRejected FeedApprovalStatus = "rejected"
)

func (e *FeedApprovalStatus) Scan(src interface{}) error {
	switch s := src.(type) {
	case []byte:
		*e = FeedApprovalStatus(s)
	case string:
		*e = FeedApprovalStatus(s)
	default:
		return fmt.Errorf("unsupported scan type for FeedApprovalStatus: %T", src)
	}
	return nil
}

type NullFeedApprovalStatus struct {
	FeedApprovalStatus FeedApprovalStatus
	Valid              bool // Valid is true if FeedApprovalStatus is not NULL
}

// Scan implements the Scanner interface.
func (ns *NullFeedApprovalStatus) Scan(value interface{}) error {
	if value == nil {
		ns.FeedApprovalStatus, ns.Valid = "", false
		return nil
	}
	ns.Valid = true
	return ns.FeedApprovalStatus.Scan(value)
}

// Value implements the driver Valuer interface.
func (ns NullFeedApprovalStatus) Value() (driver.Value, error) {
	if !ns.Valid {
		return nil, nil
	}
	return string(ns.FeedApprovalStatus), nil
}

type FeedType string

const (
	FeedTypeRss  FeedType = "rss"
	FeedTypeJson FeedType = "json"
)

func (e *FeedType) Scan(src interface{}) error {
	switch s := src.(type) {
	case []byte:
		*e = FeedType(s)
	case string:
		*e = FeedType(s)
	default:
		return fmt.Errorf("unsupported scan type for FeedType: %T", src)
	}
	return nil
}

type NullFeedType struct {
	FeedType FeedType
	Valid    bool // Valid is true if FeedType is not NULL
}

// Scan implements the Scanner interface.
func (ns *NullFeedType) Scan(value interface{}) error {
	if value == nil {
		ns.FeedType, ns.Valid = "", false
		return nil
	}
	ns.Valid = true
	return ns.FeedType.Scan(value)
}

// Value implements the driver Valuer interface.
func (ns NullFeedType) Value() (driver.Value, error) {
	if !ns.Valid {
		return nil, nil
	}
	return string(ns.FeedType), nil
}

type GoalStatus string

const (
	GoalStatusOngoing   GoalStatus = "ongoing"
	GoalStatusCompleted GoalStatus = "completed"
	GoalStatusCancelled GoalStatus = "cancelled"
)

func (e *GoalStatus) Scan(src interface{}) error {
	switch s := src.(type) {
	case []byte:
		*e = GoalStatus(s)
	case string:
		*e = GoalStatus(s)
	default:
		return fmt.Errorf("unsupported scan type for GoalStatus: %T", src)
	}
	return nil
}

type NullGoalStatus struct {
	GoalStatus GoalStatus
	Valid      bool // Valid is true if GoalStatus is not NULL
}

// Scan implements the Scanner interface.
func (ns *NullGoalStatus) Scan(value interface{}) error {
	if value == nil {
		ns.GoalStatus, ns.Valid = "", false
		return nil
	}
	ns.Valid = true
	return ns.GoalStatus.Scan(value)
}

// Value implements the driver Valuer interface.
func (ns NullGoalStatus) Value() (driver.Value, error) {
	if !ns.Valid {
		return nil, nil
	}
	return string(ns.GoalStatus), nil
}

type InvestmentTypeEnum string

const (
	InvestmentTypeEnumStock       InvestmentTypeEnum = "Stock"
	InvestmentTypeEnumBond        InvestmentTypeEnum = "Bond"
	InvestmentTypeEnumAlternative InvestmentTypeEnum = "Alternative"
)

func (e *InvestmentTypeEnum) Scan(src interface{}) error {
	switch s := src.(type) {
	case []byte:
		*e = InvestmentTypeEnum(s)
	case string:
		*e = InvestmentTypeEnum(s)
	default:
		return fmt.Errorf("unsupported scan type for InvestmentTypeEnum: %T", src)
	}
	return nil
}

type NullInvestmentTypeEnum struct {
	InvestmentTypeEnum InvestmentTypeEnum
	Valid              bool // Valid is true if InvestmentTypeEnum is not NULL
}

// Scan implements the Scanner interface.
func (ns *NullInvestmentTypeEnum) Scan(value interface{}) error {
	if value == nil {
		ns.InvestmentTypeEnum, ns.Valid = "", false
		return nil
	}
	ns.Valid = true
	return ns.InvestmentTypeEnum.Scan(value)
}

// Value implements the driver Valuer interface.
func (ns NullInvestmentTypeEnum) Value() (driver.Value, error) {
	if !ns.Valid {
		return nil, nil
	}
	return string(ns.InvestmentTypeEnum), nil
}

type InvitationStatusType string

const (
	InvitationStatusTypePending  InvitationStatusType = "pending"
	InvitationStatusTypeAccepted InvitationStatusType = "accepted"
	InvitationStatusTypeDeclined InvitationStatusType = "declined"
	InvitationStatusTypeExpired  InvitationStatusType = "expired"
)

func (e *InvitationStatusType) Scan(src interface{}) error {
	switch s := src.(type) {
	case []byte:
		*e = InvitationStatusType(s)
	case string:
		*e = InvitationStatusType(s)
	default:
		return fmt.Errorf("unsupported scan type for InvitationStatusType: %T", src)
	}
	return nil
}

type NullInvitationStatusType struct {
	InvitationStatusType InvitationStatusType
	Valid                bool // Valid is true if InvitationStatusType is not NULL
}

// Scan implements the Scanner interface.
func (ns *NullInvitationStatusType) Scan(value interface{}) error {
	if value == nil {
		ns.InvitationStatusType, ns.Valid = "", false
		return nil
	}
	ns.Valid = true
	return ns.InvitationStatusType.Scan(value)
}

// Value implements the driver Valuer interface.
func (ns NullInvitationStatusType) Value() (driver.Value, error) {
	if !ns.Valid {
		return nil, nil
	}
	return string(ns.InvitationStatusType), nil
}

type MembershipRole string

const (
	MembershipRoleMember    MembershipRole = "member"
	MembershipRoleAdmin     MembershipRole = "admin"
	MembershipRoleModerator MembershipRole = "moderator"
)

func (e *MembershipRole) Scan(src interface{}) error {
	switch s := src.(type) {
	case []byte:
		*e = MembershipRole(s)
	case string:
		*e = MembershipRole(s)
	default:
		return fmt.Errorf("unsupported scan type for MembershipRole: %T", src)
	}
	return nil
}

type NullMembershipRole struct {
	MembershipRole MembershipRole
	Valid          bool // Valid is true if MembershipRole is not NULL
}

// Scan implements the Scanner interface.
func (ns *NullMembershipRole) Scan(value interface{}) error {
	if value == nil {
		ns.MembershipRole, ns.Valid = "", false
		return nil
	}
	ns.Valid = true
	return ns.MembershipRole.Scan(value)
}

// Value implements the driver Valuer interface.
func (ns NullMembershipRole) Value() (driver.Value, error) {
	if !ns.Valid {
		return nil, nil
	}
	return string(ns.MembershipRole), nil
}

type MfaStatusType string

const (
	MfaStatusTypePending  MfaStatusType = "pending"
	MfaStatusTypeAccepted MfaStatusType = "accepted"
	MfaStatusTypeRejected MfaStatusType = "rejected"
)

func (e *MfaStatusType) Scan(src interface{}) error {
	switch s := src.(type) {
	case []byte:
		*e = MfaStatusType(s)
	case string:
		*e = MfaStatusType(s)
	default:
		return fmt.Errorf("unsupported scan type for MfaStatusType: %T", src)
	}
	return nil
}

type NullMfaStatusType struct {
	MfaStatusType MfaStatusType
	Valid         bool // Valid is true if MfaStatusType is not NULL
}

// Scan implements the Scanner interface.
func (ns *NullMfaStatusType) Scan(value interface{}) error {
	if value == nil {
		ns.MfaStatusType, ns.Valid = "", false
		return nil
	}
	ns.Valid = true
	return ns.MfaStatusType.Scan(value)
}

// Value implements the driver Valuer interface.
func (ns NullMfaStatusType) Value() (driver.Value, error) {
	if !ns.Valid {
		return nil, nil
	}
	return string(ns.MfaStatusType), nil
}

type NotificationStatus string

const (
	NotificationStatusDelivered NotificationStatus = "delivered"
	NotificationStatusRead      NotificationStatus = "read"
	NotificationStatusPending   NotificationStatus = "pending"
	NotificationStatusExpired   NotificationStatus = "expired"
)

func (e *NotificationStatus) Scan(src interface{}) error {
	switch s := src.(type) {
	case []byte:
		*e = NotificationStatus(s)
	case string:
		*e = NotificationStatus(s)
	default:
		return fmt.Errorf("unsupported scan type for NotificationStatus: %T", src)
	}
	return nil
}

type NullNotificationStatus struct {
	NotificationStatus NotificationStatus
	Valid              bool // Valid is true if NotificationStatus is not NULL
}

// Scan implements the Scanner interface.
func (ns *NullNotificationStatus) Scan(value interface{}) error {
	if value == nil {
		ns.NotificationStatus, ns.Valid = "", false
		return nil
	}
	ns.Valid = true
	return ns.NotificationStatus.Scan(value)
}

// Value implements the driver Valuer interface.
func (ns NullNotificationStatus) Value() (driver.Value, error) {
	if !ns.Valid {
		return nil, nil
	}
	return string(ns.NotificationStatus), nil
}

type RecurrenceIntervalEnum string

const (
	RecurrenceIntervalEnumDaily   RecurrenceIntervalEnum = "daily"
	RecurrenceIntervalEnumWeekly  RecurrenceIntervalEnum = "weekly"
	RecurrenceIntervalEnumMonthly RecurrenceIntervalEnum = "monthly"
	RecurrenceIntervalEnumYearly  RecurrenceIntervalEnum = "yearly"
)

func (e *RecurrenceIntervalEnum) Scan(src interface{}) error {
	switch s := src.(type) {
	case []byte:
		*e = RecurrenceIntervalEnum(s)
	case string:
		*e = RecurrenceIntervalEnum(s)
	default:
		return fmt.Errorf("unsupported scan type for RecurrenceIntervalEnum: %T", src)
	}
	return nil
}

type NullRecurrenceIntervalEnum struct {
	RecurrenceIntervalEnum RecurrenceIntervalEnum
	Valid                  bool // Valid is true if RecurrenceIntervalEnum is not NULL
}

// Scan implements the Scanner interface.
func (ns *NullRecurrenceIntervalEnum) Scan(value interface{}) error {
	if value == nil {
		ns.RecurrenceIntervalEnum, ns.Valid = "", false
		return nil
	}
	ns.Valid = true
	return ns.RecurrenceIntervalEnum.Scan(value)
}

// Value implements the driver Valuer interface.
func (ns NullRecurrenceIntervalEnum) Value() (driver.Value, error) {
	if !ns.Valid {
		return nil, nil
	}
	return string(ns.RecurrenceIntervalEnum), nil
}

type RiskToleranceType string

const (
	RiskToleranceTypeLow    RiskToleranceType = "low"
	RiskToleranceTypeMedium RiskToleranceType = "medium"
	RiskToleranceTypeHigh   RiskToleranceType = "high"
)

func (e *RiskToleranceType) Scan(src interface{}) error {
	switch s := src.(type) {
	case []byte:
		*e = RiskToleranceType(s)
	case string:
		*e = RiskToleranceType(s)
	default:
		return fmt.Errorf("unsupported scan type for RiskToleranceType: %T", src)
	}
	return nil
}

type NullRiskToleranceType struct {
	RiskToleranceType RiskToleranceType
	Valid             bool // Valid is true if RiskToleranceType is not NULL
}

// Scan implements the Scanner interface.
func (ns *NullRiskToleranceType) Scan(value interface{}) error {
	if value == nil {
		ns.RiskToleranceType, ns.Valid = "", false
		return nil
	}
	ns.Valid = true
	return ns.RiskToleranceType.Scan(value)
}

// Value implements the driver Valuer interface.
func (ns NullRiskToleranceType) Value() (driver.Value, error) {
	if !ns.Valid {
		return nil, nil
	}
	return string(ns.RiskToleranceType), nil
}

type TimeHorizonType string

const (
	TimeHorizonTypeShort  TimeHorizonType = "short"
	TimeHorizonTypeMedium TimeHorizonType = "medium"
	TimeHorizonTypeLong   TimeHorizonType = "long"
)

func (e *TimeHorizonType) Scan(src interface{}) error {
	switch s := src.(type) {
	case []byte:
		*e = TimeHorizonType(s)
	case string:
		*e = TimeHorizonType(s)
	default:
		return fmt.Errorf("unsupported scan type for TimeHorizonType: %T", src)
	}
	return nil
}

type NullTimeHorizonType struct {
	TimeHorizonType TimeHorizonType
	Valid           bool // Valid is true if TimeHorizonType is not NULL
}

// Scan implements the Scanner interface.
func (ns *NullTimeHorizonType) Scan(value interface{}) error {
	if value == nil {
		ns.TimeHorizonType, ns.Valid = "", false
		return nil
	}
	ns.Valid = true
	return ns.TimeHorizonType.Scan(value)
}

// Value implements the driver Valuer interface.
func (ns NullTimeHorizonType) Value() (driver.Value, error) {
	if !ns.Valid {
		return nil, nil
	}
	return string(ns.TimeHorizonType), nil
}

type TrackingTypeEnum string

const (
	TrackingTypeEnumMonthly TrackingTypeEnum = "monthly"
	TrackingTypeEnumBonus   TrackingTypeEnum = "bonus"
	TrackingTypeEnumOther   TrackingTypeEnum = "other"
)

func (e *TrackingTypeEnum) Scan(src interface{}) error {
	switch s := src.(type) {
	case []byte:
		*e = TrackingTypeEnum(s)
	case string:
		*e = TrackingTypeEnum(s)
	default:
		return fmt.Errorf("unsupported scan type for TrackingTypeEnum: %T", src)
	}
	return nil
}

type NullTrackingTypeEnum struct {
	TrackingTypeEnum TrackingTypeEnum
	Valid            bool // Valid is true if TrackingTypeEnum is not NULL
}

// Scan implements the Scanner interface.
func (ns *NullTrackingTypeEnum) Scan(value interface{}) error {
	if value == nil {
		ns.TrackingTypeEnum, ns.Valid = "", false
		return nil
	}
	ns.Valid = true
	return ns.TrackingTypeEnum.Scan(value)
}

// Value implements the driver Valuer interface.
func (ns NullTrackingTypeEnum) Value() (driver.Value, error) {
	if !ns.Valid {
		return nil, nil
	}
	return string(ns.TrackingTypeEnum), nil
}

type TransactionTypeEnum string

const (
	TransactionTypeEnumBuy   TransactionTypeEnum = "buy"
	TransactionTypeEnumSell  TransactionTypeEnum = "sell"
	TransactionTypeEnumOther TransactionTypeEnum = "other"
)

func (e *TransactionTypeEnum) Scan(src interface{}) error {
	switch s := src.(type) {
	case []byte:
		*e = TransactionTypeEnum(s)
	case string:
		*e = TransactionTypeEnum(s)
	default:
		return fmt.Errorf("unsupported scan type for TransactionTypeEnum: %T", src)
	}
	return nil
}

type NullTransactionTypeEnum struct {
	TransactionTypeEnum TransactionTypeEnum
	Valid               bool // Valid is true if TransactionTypeEnum is not NULL
}

// Scan implements the Scanner interface.
func (ns *NullTransactionTypeEnum) Scan(value interface{}) error {
	if value == nil {
		ns.TransactionTypeEnum, ns.Valid = "", false
		return nil
	}
	ns.Valid = true
	return ns.TransactionTypeEnum.Scan(value)
}

// Value implements the driver Valuer interface.
func (ns NullTransactionTypeEnum) Value() (driver.Value, error) {
	if !ns.Valid {
		return nil, nil
	}
	return string(ns.TransactionTypeEnum), nil
}

type AlternativeInvestment struct {
	ID                 int64
	UserID             int64
	InvestmentType     string
	InvestmentName     sql.NullString
	IsBusiness         bool
	Quantity           sql.NullString
	AnnualRevenue      sql.NullString
	AcquiredAt         time.Time
	ProfitMargin       sql.NullString
	Valuation          string
	ValuationUpdatedAt sql.NullTime
	Location           sql.NullString
	CreatedAt          sql.NullTime
	UpdatedAt          sql.NullTime
}

type Award struct {
	ID          int32
	Code        string
	Description string
	Point       int32
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

type BondInvestment struct {
	ID            int64
	UserID        int64
	BondSymbol    string
	Quantity      string
	PurchasePrice string
	CurrentValue  string
	CouponRate    sql.NullString
	MaturityDate  time.Time
	PurchaseDate  time.Time
	CreatedAt     sql.NullTime
	UpdatedAt     sql.NullTime
}

type Budget struct {
	ID             int64
	UserID         int64
	Name           string
	IsStrict       bool
	Category       string
	TotalAmount    string
	CurrencyCode   string
	ConversionRate string
	Description    sql.NullString
	CreatedAt      time.Time
	UpdatedAt      time.Time
}

type Debt struct {
	ID                     int64
	UserID                 int64
	Name                   string
	Amount                 string
	RemainingBalance       string
	InterestRate           sql.NullString
	Description            sql.NullString
	DueDate                time.Time
	MinimumPayment         string
	CreatedAt              sql.NullTime
	UpdatedAt              sql.NullTime
	NextPaymentDate        time.Time
	EstimatedPayoffDate    sql.NullTime
	AccruedInterest        sql.NullString
	InterestLastCalculated sql.NullTime
	LastPaymentDate        sql.NullTime
	TotalInterestPaid      sql.NullString
}

type Debtpayment struct {
	ID               int64
	DebtID           int64
	UserID           int64
	PaymentAmount    string
	PaymentDate      time.Time
	InterestPayment  string
	PrincipalPayment string
	CreatedAt        sql.NullTime
}

type Expense struct {
	ID           int64
	UserID       int64
	BudgetID     int64
	Name         string
	Category     string
	Amount       string
	IsRecurring  bool
	Description  sql.NullString
	DateOccurred time.Time
	CreatedAt    sql.NullTime
	UpdatedAt    sql.NullTime
}

type FavoritePost struct {
	ID        int64
	PostID    int64
	FeedID    int64
	UserID    int64
	CreatedAt time.Time
}

type Feed struct {
	ID              int64
	UserID          int64
	Name            string
	Url             string
	ImgUrl          sql.NullString
	FeedType        FeedType
	FeedCategory    string
	FeedDescription sql.NullString
	IsHidden        bool
	ApprovalStatus  FeedApprovalStatus
	Version         int32
	CreatedAt       time.Time
	UpdatedAt       time.Time
	LastFetchedAt   sql.NullTime
}

type Goal struct {
	ID                  int64
	UserID              int64
	BudgetID            sql.NullInt64
	Name                string
	CurrentAmount       sql.NullString
	TargetAmount        string
	MonthlyContribution string
	StartDate           time.Time
	EndDate             time.Time
	Status              GoalStatus
	CreatedAt           time.Time
	UpdatedAt           time.Time
}

type GoalPlan struct {
	ID                  int64
	UserID              int64
	Name                string
	Description         sql.NullString
	TargetAmount        sql.NullString
	MonthlyContribution sql.NullString
	DurationInMonths    sql.NullInt32
	IsStrict            bool
	CreatedAt           sql.NullTime
	UpdatedAt           sql.NullTime
}

type GoalTracking struct {
	ID                    int64
	UserID                int64
	GoalID                sql.NullInt64
	TrackingDate          time.Time
	ContributedAmount     string
	TrackingType          TrackingTypeEnum
	CreatedAt             sql.NullTime
	UpdatedAt             sql.NullTime
	TruncatedTrackingDate sql.NullTime
}

type Group struct {
	ID             int64
	CreatorUserID  sql.NullInt64
	GroupImageUrl  string
	Name           string
	IsPrivate      sql.NullBool
	MaxMemberCount sql.NullInt32
	Description    sql.NullString
	ActivityCount  sql.NullInt32
	LastActivityAt sql.NullTime
	CreatedAt      sql.NullTime
	UpdatedAt      sql.NullTime
	Version        sql.NullInt32
}

type GroupExpense struct {
	ID          int64
	GroupID     sql.NullInt64
	MemberID    sql.NullInt64
	Amount      string
	Description sql.NullString
	Category    sql.NullString
	CreatedAt   sql.NullTime
	UpdatedAt   sql.NullTime
}

type GroupGoal struct {
	ID            int64
	GroupID       int64
	CreatorUserID int64
	GoalName      string
	TargetAmount  string
	CurrentAmount sql.NullString
	StartDate     time.Time
	Deadline      time.Time
	Description   string
	Status        GoalStatus
	CreatedAt     time.Time
	UpdatedAt     time.Time
}

type GroupInvitation struct {
	ID               int64
	GroupID          sql.NullInt64
	InviterUserID    sql.NullInt64
	InviteeUserEmail string
	Status           InvitationStatusType
	SentAt           sql.NullTime
	RespondedAt      sql.NullTime
	ExpirationDate   time.Time
}

type GroupMembership struct {
	ID           int64
	GroupID      sql.NullInt64
	UserID       sql.NullInt64
	Status       NullMfaStatusType
	ApprovalTime sql.NullTime
	RequestTime  sql.NullTime
	Role         NullMembershipRole
	CreatedAt    sql.NullTime
	UpdatedAt    sql.NullTime
}

type GroupTransaction struct {
	ID          int64
	GoalID      sql.NullInt64
	MemberID    sql.NullInt64
	Amount      string
	Description sql.NullString
	CreatedAt   sql.NullTime
	UpdatedAt   sql.NullTime
}

type Income struct {
	ID                   int64
	UserID               int64
	Source               string
	OriginalCurrencyCode string
	AmountOriginal       string
	Amount               string
	ExchangeRate         string
	Description          sql.NullString
	DateReceived         time.Time
	CreatedAt            sql.NullTime
	UpdatedAt            sql.NullTime
}

type InvestmentTransaction struct {
	ID                int64
	UserID            int64
	InvestmentType    InvestmentTypeEnum
	InvestmentID      int64
	TransactionType   TransactionTypeEnum
	TransactionDate   time.Time
	TransactionAmount string
	Quantity          string
	CreatedAt         sql.NullTime
	UpdatedAt         sql.NullTime
}

type Notification struct {
	ID               int64
	UserID           int64
	Message          string
	NotificationType string
	Status           NotificationStatus
	CreatedAt        time.Time
	UpdatedAt        time.Time
	ReadAt           sql.NullTime
	ExpiresAt        sql.NullTime
	Meta             pqtype.NullRawMessage
	RedisKey         sql.NullString
}

type RecurringExpense struct {
	ID                 int64
	UserID             int64
	BudgetID           int64
	Amount             string
	Name               string
	Description        sql.NullString
	RecurrenceInterval RecurrenceIntervalEnum
	ProjectedAmount    string
	NextOccurrence     time.Time
	CreatedAt          sql.NullTime
	UpdatedAt          sql.NullTime
}

type RssfeedPost struct {
	ID                 int64
	CreatedAt          time.Time
	UpdatedAt          time.Time
	Channeltitle       string
	Channelurl         sql.NullString
	Channeldescription sql.NullString
	Channellanguage    sql.NullString
	Itemtitle          string
	Itemdescription    sql.NullString
	Itemcontent        sql.NullString
	ItempublishedAt    time.Time
	Itemurl            string
	ImgUrl             string
	FeedID             int64
}

type StockInvestment struct {
	ID                     int64
	UserID                 int64
	StockSymbol            string
	Quantity               string
	PurchasePrice          string
	CurrentValue           string
	Sector                 sql.NullString
	PurchaseDate           time.Time
	DividendYield          sql.NullString
	DividendYieldUpdatedAt sql.NullTime
	CreatedAt              sql.NullTime
	UpdatedAt              sql.NullTime
}

type Token struct {
	Hash   []byte
	UserID int64
	Expiry time.Time
	Scope  string
}

type User struct {
	ID               int64
	FirstName        string
	LastName         string
	Email            string
	ProfileAvatarUrl string
	Password         []byte
	RoleLevel        string
	PhoneNumber      string
	Activated        bool
	Version          int32
	CreatedAt        time.Time
	UpdatedAt        time.Time
	LastLogin        time.Time
	ProfileCompleted bool
	Dob              time.Time
	Address          sql.NullString
	CountryCode      sql.NullString
	CurrencyCode     sql.NullString
	MfaEnabled       bool
	MfaSecret        sql.NullString
	MfaStatus        NullMfaStatusType
	MfaLastChecked   sql.NullTime
	RiskTolerance    NullRiskToleranceType
	TimeHorizon      NullTimeHorizonType
}

type UserAward struct {
	UserID    int64
	AwardID   int32
	CreatedAt time.Time
}
