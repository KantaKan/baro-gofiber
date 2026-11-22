package staff

import (
	"errors"
	"testing"
	"time"

	"gofiber-baro/internal/domain"
	"gofiber-baro/pkg/utils"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

type memStore struct {
	users   map[primitive.ObjectID]*domain.User
	invites map[primitive.ObjectID]*Invite
}

func newMem() *memStore {
	return &memStore{users: map[primitive.ObjectID]*domain.User{}, invites: map[primitive.ObjectID]*Invite{}}
}
func (m *memStore) Members() ([]domain.User, error) {
	var out []domain.User
	for _, u := range m.users {
		if u.Role == "admin" {
			out = append(out, *u)
		}
	}
	return out, nil
}
func (m *memStore) UserByEmail(e string) (*domain.User, error) {
	for _, u := range m.users {
		if u.Email == e {
			return u, nil
		}
	}
	return nil, nil
}
func (m *memStore) UserByID(id primitive.ObjectID) (*domain.User, error) { return m.users[id], nil }
func (m *memStore) CreateUser(u *domain.User) error {
	u.ID = primitive.NewObjectID()
	m.users[u.ID] = u
	return nil
}
func (m *memStore) SetStaff(id primitive.ObjectID, role, staffRole string, cohorts []int, groups []string) error {
	u := m.users[id]
	u.Role, u.StaffRole, u.StaffCohorts, u.StaffGroups = role, staffRole, cohorts, groups
	return nil
}
func (m *memStore) SaveInvite(i *Invite) error {
	if i.ID.IsZero() {
		i.ID = primitive.NewObjectID()
	}
	c := *i
	m.invites[i.ID] = &c
	return nil
}
func (m *memStore) Invites() ([]Invite, error) {
	var out []Invite
	for _, i := range m.invites {
		if i.UsedAt == nil {
			out = append(out, *i)
		}
	}
	return out, nil
}
func (m *memStore) InviteByID(id primitive.ObjectID) (*Invite, error) { return m.invites[id], nil }
func (m *memStore) InviteByHash(h string) (*Invite, error) {
	for _, i := range m.invites {
		if i.TokenHash == h {
			c := *i
			return &c, nil
		}
	}
	return nil, nil
}
func (m *memStore) DeleteInvite(id primitive.ObjectID) error { delete(m.invites, id); return nil }
func (m *memStore) MarkUsed(id primitive.ObjectID, at time.Time) (bool, error) {
	i := m.invites[id]
	if i == nil || i.UsedAt != nil {
		return false, nil
	}
	i.UsedAt = &at
	return true, nil
}

func setup() (*Team, *memStore, primitive.ObjectID, *time.Time) {
	m := newMem()
	now := time.Date(2026, 11, 22, 3, 0, 0, 0, time.UTC)
	t := NewTeam(m)
	t.now = func() time.Time { return now }
	owner := &domain.User{FirstName: "Kan", Email: "kan@x.co", Role: "admin"}
	_ = m.CreateUser(owner)
	return t, m, owner.ID, &now
}

func TestInviteNewAccount(t *testing.T) {
	team, m, owner, _ := setup()
	tok, inv, err := team.Invite(owner, " Mint@X.co ", TA, []int{13}, []string{"B"})
	if err != nil || len(tok) < 40 || inv.Email != "mint@x.co" || inv.TokenHash == tok {
		t.Fatalf("invite: %v %+v", err, inv)
	}
	got, existing, err := team.Lookup(tok)
	if err != nil || existing || got.Role != TA {
		t.Fatalf("lookup: %v %v %+v", err, existing, got)
	}
	if err := team.Accept(tok, "Mint", "S", "short"); !errors.Is(err, ErrWeakPassword) {
		t.Errorf("weak password: %v", err)
	}
	if err := team.Accept(tok, "Mint", "S", "longenough1"); err != nil {
		t.Fatalf("accept: %v", err)
	}
	u, _ := m.UserByEmail("mint@x.co")
	if u == nil || u.Role != "admin" || u.StaffRole != TA || u.StaffCohorts[0] != 13 || !utils.CheckPasswordHash("longenough1", u.Password) {
		t.Fatalf("user: %+v", u)
	}
	if _, _, err := team.Lookup(tok); !errors.Is(err, ErrInviteInvalid) {
		t.Error("used invite still works")
	}
	if err := team.Accept(tok, "Mint", "S", "longenough1"); !errors.Is(err, ErrInviteInvalid) {
		t.Error("reuse accepted")
	}
	if _, _, err := team.Lookup("guess"); !errors.Is(err, ErrInviteInvalid) {
		t.Error("guess accepted")
	}
}

func TestInviteExpiresAndRenews(t *testing.T) {
	team, _, owner, now := setup()
	tok, inv, _ := team.Invite(owner, "a@x.co", Viewer, []int{13}, nil)
	*now = now.Add(8 * 24 * time.Hour)
	if _, _, err := team.Lookup(tok); !errors.Is(err, ErrInviteInvalid) {
		t.Fatal("expired invite works")
	}
	if _, _, err := team.Renew(primitive.NewObjectID(), inv.ID); !errors.Is(err, ErrNotOwner) {
		t.Fatalf("a non-owner renewed an invite: %v", err)
	}
	tok2, _, err := team.Renew(owner, inv.ID)
	if err != nil || tok2 == tok {
		t.Fatalf("renew: %v", err)
	}
	if _, _, err := team.Lookup(tok2); err != nil {
		t.Errorf("renewed: %v", err)
	}
	if _, _, err := team.Lookup(tok); !errors.Is(err, ErrInviteInvalid) {
		t.Error("old token still works after renew")
	}
	if _, _, err := team.Invite(owner, "b@x.co", Owner, nil, nil); !errors.Is(err, ErrBadRole) {
		t.Error("invite as owner allowed")
	}
}

func TestOnlyOwnerCancels(t *testing.T) {
	team, m, owner, _ := setup()
	_, inv, _ := team.Invite(owner, "a@x.co", Viewer, []int{13}, nil)
	adm := &domain.User{FirstName: "A", Email: "ad@x.co", Role: "admin", StaffRole: Admin}
	_ = m.CreateUser(adm)
	if err := team.CancelInvite(adm.ID, inv.ID); !errors.Is(err, ErrNotOwner) || len(m.invites) != 1 {
		t.Fatalf("admin cancelled an invite: %v", err)
	}
	if err := team.CancelInvite(owner, inv.ID); err != nil || len(m.invites) != 0 {
		t.Fatalf("owner cancel: %v", err)
	}
}

func TestInviteExistingAccount(t *testing.T) {
	team, m, owner, _ := setup()
	hash, _ := utils.HashPassword("oldpassword")
	l := &domain.User{FirstName: "Fah", Email: "fah@x.co", Role: "learner", Password: hash}
	_ = m.CreateUser(l)
	tok, _, _ := team.Invite(owner, "fah@x.co", Admin, []int{13}, nil)
	if _, existing, _ := team.Lookup(tok); !existing {
		t.Fatal("should be existing")
	}
	if err := team.Accept(tok, "", "", "wrong"); !errors.Is(err, ErrWrongPassword) || m.users[l.ID].Role != "learner" {
		t.Fatalf("wrong password: %v %s", err, m.users[l.ID].Role)
	}
	if err := team.Accept(tok, "", "", "oldpassword"); err != nil || m.users[l.ID].Role != "admin" || m.users[l.ID].StaffRole != Admin {
		t.Fatalf("upgrade: %v %+v", err, m.users[l.ID])
	}
}

func TestChangeAndRemove(t *testing.T) {
	team, m, owner, _ := setup()
	ta := &domain.User{FirstName: "T", Email: "t@x.co", Role: "admin", StaffRole: TA}
	_ = m.CreateUser(ta)
	if err := team.Update(owner, owner, Viewer, nil, nil); !errors.Is(err, ErrSelf) {
		t.Error("self demote allowed")
	}
	if err := team.Remove(owner, owner); !errors.Is(err, ErrSelf) {
		t.Error("self remove allowed")
	}
	if err := team.Update(owner, ta.ID, Admin, []int{13, 14}, nil); err != nil || m.users[ta.ID].StaffRole != Admin {
		t.Fatalf("update: %v", err)
	}
	if err := team.Update(owner, ta.ID, "god", nil, nil); !errors.Is(err, ErrBadRole) {
		t.Error("bad role")
	}
	// A second owner can't remove the last other owner if that leaves none: here owner stays, fine.
	if err := team.Remove(owner, ta.ID); err != nil || m.users[ta.ID].Role != "disabled" {
		t.Fatalf("remove: %v", err)
	}
	ms, _ := team.Members()
	if len(ms) != 1 {
		t.Errorf("removed member still listed: %d", len(ms))
	}
}

func TestLastOwner(t *testing.T) {
	team, m, owner, _ := setup()
	other := &domain.User{FirstName: "A", Email: "a@x.co", Role: "admin", StaffRole: Admin}
	_ = m.CreateUser(other)
	// make 'other' owner, then demote original owner via other: allowed (still one owner)
	if err := team.Update(owner, other.ID, Owner, nil, nil); err != nil {
		t.Fatal(err)
	}
	if err := team.Update(other.ID, owner, Admin, nil, nil); err != nil {
		t.Fatalf("demote with another owner left: %v", err)
	}
	if err := team.Update(owner, other.ID, Admin, nil, nil); err == nil {
		t.Error("admin changed an owner")
	}
}
