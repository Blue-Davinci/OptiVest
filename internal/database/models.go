// Code generated by sqlc. DO NOT EDIT.
// versions:
//   sqlc v1.26.0

package database

import (
	"database/sql"
	"database/sql/driver"
	"fmt"
	"time"
)

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
}
