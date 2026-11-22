package staff

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"strings"
	"time"

	"gofiber-baro/internal/domain"
	"gofiber-baro/pkg/utils"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

const InviteTTL = 7 * 24 * time.Hour

var (
	ErrInviteInvalid = errors.New("this invite link has expired or was already used")
	ErrWrongPassword = errors.New("that password doesn't match the existing account")
	ErrWeakPassword  = errors.New("password must be at least 8 characters")
	ErrBadRole       = errors.New("unknown role")
	ErrBadEmail      = errors.New("a valid email is required")
	ErrSelf          = errors.New("you can't change or remove your own access")
	ErrNotOwner      = errors.New("only an Owner can manage the team")
	ErrNotMember     = errors.New("not a team member")
	ErrAlreadyStaff  = errors.New("this person is already on the team")
)

// Invite is a pending staff invite; only a hash of its token is stored.
type Invite struct {
	ID        primitive.ObjectID `bson:"_id,omitempty" json:"id"`
	TokenHash string             `bson:"token_hash" json:"-"`
	Email     string             `bson:"email" json:"email"`
	Role      string             `bson:"role" json:"role"`
	Cohorts   []int              `bson:"cohorts" json:"cohorts"`
	Groups    []string           `bson:"groups" json:"groups"`
	InvitedBy primitive.ObjectID `bson:"invited_by" json:"-"`
	ExpiresAt time.Time          `bson:"expires_at" json:"expires_at"`
	UsedAt    *time.Time         `bson:"used_at,omitempty" json:"-"`
}

// Store is what the team service needs from the database.
type Store interface {
	Members() ([]domain.User, error)
	UserByEmail(email string) (*domain.User, error)
	UserByID(id primitive.ObjectID) (*domain.User, error)
	CreateUser(u *domain.User) error
	SetStaff(id primitive.ObjectID, role, staffRole string, cohorts []int, groups []string) error
	SaveInvite(i *Invite) error
	Invites() ([]Invite, error)
	InviteByID(id primitive.ObjectID) (*Invite, error)
	InviteByHash(hash string) (*Invite, error)
	DeleteInvite(id primitive.ObjectID) error
	MarkUsed(id primitive.ObjectID, at time.Time) (bool, error)
}

type Team struct {
	s   Store
	now func() time.Time
}

func NewTeam(s Store) *Team { return &Team{s: s, now: time.Now} }

func hashToken(t string) string {
	h := sha256.Sum256([]byte(t))
	return hex.EncodeToString(h[:])
}

func newToken() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}

func clean(role string, cohorts []int, groups []string) ([]int, []string) {
	if cohorts == nil {
		cohorts = []int{}
	}
	if role != TA || groups == nil {
		groups = []string{}
	}
	return cohorts, groups
}

func (t *Team) requireOwner(actor primitive.ObjectID) error {
	u, err := t.s.UserByID(actor)
	if err != nil || u == nil || Effective(u.Role, u.StaffRole) != Owner {
		return ErrNotOwner
	}
	return nil
}

// Members lists current staff (removed members are not listed).
func (t *Team) Members() ([]domain.User, error) { return t.s.Members() }

// Invites lists invites not yet accepted (expired ones included, so they can be renewed).
func (t *Team) Invites() ([]Invite, error) { return t.s.Invites() }

// Invite creates an invite and returns the raw token (shown once, as a link).
func (t *Team) Invite(actor primitive.ObjectID, email, role string, cohorts []int, groups []string) (string, *Invite, error) {
	if err := t.requireOwner(actor); err != nil {
		return "", nil, err
	}
	email = strings.ToLower(strings.TrimSpace(email))
	if !strings.Contains(email, "@") || len(email) > 200 {
		return "", nil, ErrBadEmail
	}
	if role != Admin && role != TA && role != Viewer {
		return "", nil, ErrBadRole
	}
	if u, _ := t.s.UserByEmail(email); u != nil && u.Role == "admin" {
		return "", nil, ErrAlreadyStaff
	}
	tok, err := newToken()
	if err != nil {
		return "", nil, err
	}
	cohorts, groups = clean(role, cohorts, groups)
	inv := &Invite{TokenHash: hashToken(tok), Email: email, Role: role, Cohorts: cohorts, Groups: groups, InvitedBy: actor, ExpiresAt: t.now().Add(InviteTTL)}
	if err := t.s.SaveInvite(inv); err != nil {
		return "", nil, err
	}
	return tok, inv, nil
}

// Renew issues a new link for a pending invite; the old link stops working.
func (t *Team) Renew(actor, id primitive.ObjectID) (string, *Invite, error) {
	if err := t.requireOwner(actor); err != nil {
		return "", nil, err
	}
	inv, err := t.s.InviteByID(id)
	if err != nil || inv == nil || inv.UsedAt != nil {
		return "", nil, ErrInviteInvalid
	}
	tok, err := newToken()
	if err != nil {
		return "", nil, err
	}
	inv.TokenHash, inv.ExpiresAt = hashToken(tok), t.now().Add(InviteTTL)
	if err := t.s.SaveInvite(inv); err != nil {
		return "", nil, err
	}
	return tok, inv, nil
}

func (t *Team) CancelInvite(actor, id primitive.ObjectID) error {
	if err := t.requireOwner(actor); err != nil {
		return err
	}
	return t.s.DeleteInvite(id)
}

func (t *Team) valid(token string) (*Invite, error) {
	if len(token) < 20 {
		return nil, ErrInviteInvalid
	}
	inv, err := t.s.InviteByHash(hashToken(token))
	if err != nil || inv == nil || inv.UsedAt != nil || !t.now().Before(inv.ExpiresAt) {
		return nil, ErrInviteInvalid
	}
	return inv, nil
}

// Lookup returns a valid invite and whether its email already has an account.
func (t *Team) Lookup(token string) (*Invite, bool, error) {
	inv, err := t.valid(token)
	if err != nil {
		return nil, false, err
	}
	u, _ := t.s.UserByEmail(inv.Email)
	return inv, u != nil, nil
}

// Accept creates the staff account, or upgrades an existing one after checking its password.
func (t *Team) Accept(token, first, last, password string) error {
	inv, err := t.valid(token)
	if err != nil {
		return err
	}
	existing, _ := t.s.UserByEmail(inv.Email)
	if existing != nil {
		if !utils.CheckPasswordHash(password, existing.Password) {
			return ErrWrongPassword
		}
	} else {
		if len(password) < 8 {
			return ErrWeakPassword
		}
		if strings.TrimSpace(first) == "" {
			return errors.New("first name is required")
		}
	}
	// Claim the invite first, so two tabs can't both use it.
	ok, err := t.s.MarkUsed(inv.ID, t.now())
	if err != nil || !ok {
		return ErrInviteInvalid
	}
	if existing != nil {
		return t.s.SetStaff(existing.ID, "admin", inv.Role, inv.Cohorts, inv.Groups)
	}
	hash, err := utils.HashPassword(password)
	if err != nil {
		return err
	}
	cohort := 0
	if len(inv.Cohorts) > 0 {
		cohort = inv.Cohorts[0]
	}
	return t.s.CreateUser(&domain.User{FirstName: strings.TrimSpace(first), LastName: strings.TrimSpace(last), Email: inv.Email, Password: hash, Role: "admin", CohortNumber: cohort, StaffRole: inv.Role, StaffCohorts: inv.Cohorts, StaffGroups: inv.Groups, Reflections: []domain.Reflection{}})
}

func (t *Team) member(actor, id primitive.ObjectID) (*domain.User, error) {
	if err := t.requireOwner(actor); err != nil {
		return nil, err
	}
	if actor == id {
		return nil, ErrSelf
	}
	u, err := t.s.UserByID(id)
	if err != nil || u == nil || u.Role != "admin" {
		return nil, ErrNotMember
	}
	return u, nil
}

// Update changes a member's role and scope. Owners can't change themselves, so an Owner always remains.
func (t *Team) Update(actor, id primitive.ObjectID, role string, cohorts []int, groups []string) error {
	if !Valid(role) {
		return ErrBadRole
	}
	if _, err := t.member(actor, id); err != nil {
		return err
	}
	cohorts, groups = clean(role, cohorts, groups)
	return t.s.SetStaff(id, "admin", role, cohorts, groups)
}

// Remove ends a member's access (role "disabled"; login is refused).
func (t *Team) Remove(actor, id primitive.ObjectID) error {
	u, err := t.member(actor, id)
	if err != nil {
		return err
	}
	return t.s.SetStaff(id, "disabled", u.StaffRole, u.StaffCohorts, u.StaffGroups)
}
