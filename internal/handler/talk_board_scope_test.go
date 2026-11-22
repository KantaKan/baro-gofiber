package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"gofiber-baro/internal/domain"
	"gofiber-baro/internal/service/staff"

	"github.com/gofiber/fiber/v2"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

func TestBoardPostAccess(t *testing.T) {
	learner13 := boardActor{ownCohort: 13}
	ta13 := boardActor{role: staff.TA, cohorts: []int{13}}
	viewer13 := boardActor{role: staff.Viewer, cohorts: []int{13}}
	owner := boardActor{role: staff.Owner}
	cases := []struct {
		name  string
		a     boardActor
		post  int
		write bool
		ok    bool
	}{
		{"learner reads own cohort", learner13, 13, false, true},
		{"learner writes own cohort", learner13, 13, true, true},
		{"learner can't read another cohort", learner13, 14, false, false},
		{"learner can't write another cohort", learner13, 14, true, false},
		{"TA reads in scope", ta13, 13, false, true},
		{"TA writes in scope", ta13, 13, true, true},
		{"TA can't read out of scope", ta13, 14, false, false},
		{"TA can't write out of scope", ta13, 14, true, false},
		{"Viewer reads in scope", viewer13, 13, false, true},
		{"Viewer can't write (read-only)", viewer13, 13, true, false},
		{"Owner reads anywhere", owner, 14, false, true},
		{"Owner writes anywhere", owner, 14, true, true},
		{"learner with no cohort reads nothing", boardActor{ownCohort: 0}, 0, false, false},
	}
	for _, tc := range cases {
		if got := boardPostAccess(tc.a, tc.post, tc.write); got != tc.ok {
			t.Errorf("%s: got %v, want %v", tc.name, got, tc.ok)
		}
	}
}

// ---- handler tests with small fakes ----

type fakeBoardRepo struct {
	post          domain.Post
	comments      []domain.Comment
	postReactions int
	pulledComment bool
	commentReacts int
}

func (f *fakeBoardRepo) InsertPost(context.Context, *domain.Post) error { return nil }
func (f *fakeBoardRepo) FindPosts(context.Context, domain.PostFilter, interface{}) ([]domain.Post, error) {
	return nil, nil
}
func (f *fakeBoardRepo) FindByID(_ context.Context, id primitive.ObjectID) (*domain.Post, error) {
	if id != f.post.ID {
		return nil, errors.New("not found")
	}
	p := f.post
	return &p, nil
}
func (f *fakeBoardRepo) UpdatePost(context.Context, primitive.ObjectID, interface{}) error {
	return nil
}
func (f *fakeBoardRepo) DeletePost(context.Context, primitive.ObjectID) error { return nil }
func (f *fakeBoardRepo) DeleteComment(context.Context, primitive.ObjectID, primitive.ObjectID) error {
	return nil
}
func (f *fakeBoardRepo) AddComment(_ context.Context, _ primitive.ObjectID, c domain.Comment) error {
	f.comments = append(f.comments, c)
	return nil
}
func (f *fakeBoardRepo) AddReaction(context.Context, primitive.ObjectID, domain.Reaction) error {
	f.postReactions++
	return nil
}
func (f *fakeBoardRepo) AddCommentReaction(context.Context, primitive.ObjectID, primitive.ObjectID, domain.Reaction) error {
	f.commentReacts++
	return nil
}
func (f *fakeBoardRepo) RemoveCommentReaction(context.Context, primitive.ObjectID, primitive.ObjectID, primitive.ObjectID) error {
	f.pulledComment = true
	return nil
}
func (f *fakeBoardRepo) Exists(context.Context, primitive.ObjectID) (bool, error) { return true, nil }

type fakeUsers map[string]*domain.User

func (f fakeUsers) GetUserByID(id string) (*domain.User, error) {
	if u, ok := f[id]; ok {
		return u, nil
	}
	return nil, errors.New("no user")
}

var (
	learner13ID = primitive.NewObjectID()
	learner14ID = primitive.NewObjectID()
	ta13ID      = primitive.NewObjectID()
	viewerID    = primitive.NewObjectID()
	ownerID     = primitive.NewObjectID()
)

func boardTestApp(repo *fakeBoardRepo) *fiber.App {
	users := fakeUsers{
		learner13ID.Hex(): {ID: learner13ID, Role: "learner", CohortNumber: 13, ZoomName: "01_Fah"},
		learner14ID.Hex(): {ID: learner14ID, Role: "learner", CohortNumber: 14},
		ta13ID.Hex():      {ID: ta13ID, Role: "admin", StaffRole: staff.TA, StaffCohorts: []int{13}, CohortNumber: 0, FirstName: "Kan"},
		viewerID.Hex():    {ID: viewerID, Role: "admin", StaffRole: staff.Viewer, StaffCohorts: []int{13}},
		ownerID.Hex():     {ID: ownerID, Role: "admin", StaffRole: staff.Owner},
	}
	h := &TalkBoardHandler{repo: repo, userService: users}
	app := fiber.New()
	app.Use(func(c *fiber.Ctx) error {
		c.Locals("userID", c.Get("X-User"))
		return c.Next()
	})
	app.Get("/board/posts/:postId", h.GetPost)
	app.Post("/board/posts/:postId/comments", h.AddComment)
	app.Post("/board/posts/:postId/reactions", h.AddReactionToPost)
	app.Delete("/board/posts/:postId/reactions", h.RemoveReactionFromPost)
	app.Post("/board/posts/:postId/comments/:commentId/reactions", h.AddReactionToComment)
	app.Delete("/board/posts/:postId/comments/:commentId/reactions", h.RemoveReactionFromComment)
	return app
}

func call(t *testing.T, app *fiber.App, method, path string, as primitive.ObjectID, body any) int {
	t.Helper()
	var buf bytes.Buffer
	if body != nil {
		_ = json.NewEncoder(&buf).Encode(body)
	}
	req := httptest.NewRequest(method, path, &buf)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-User", as.Hex())
	res, err := app.Test(req)
	if err != nil {
		t.Fatal(err)
	}
	return res.StatusCode
}

func TestBoardRoutesFollowScope(t *testing.T) {
	commentID := primitive.NewObjectID()
	repo := &fakeBoardRepo{post: domain.Post{ID: primitive.NewObjectID(), Cohort: 13, Comments: []domain.Comment{{ID: commentID}}}}
	app := boardTestApp(repo)
	p := "/board/posts/" + repo.post.ID.Hex()
	c := p + "/comments/" + commentID.Hex() + "/reactions"
	react := map[string]string{"reaction": "👍"}
	reply := map[string]string{"content": "hi"}

	cases := []struct {
		name   string
		method string
		path   string
		as     primitive.ObjectID
		body   any
		want   int
	}{
		{"learner reads own cohort post", http.MethodGet, p, learner13ID, nil, 200},
		{"other-cohort learner can't read", http.MethodGet, p, learner14ID, nil, 403},
		{"other-cohort learner can't react", http.MethodPost, p + "/reactions", learner14ID, react, 403},
		{"TA in scope reads", http.MethodGet, p, ta13ID, nil, 200},
		{"TA in scope replies", http.MethodPost, p + "/comments", ta13ID, reply, 200},
		{"Viewer reads", http.MethodGet, p, viewerID, nil, 200},
		{"Viewer can't reply", http.MethodPost, p + "/comments", viewerID, reply, 403},
		{"Viewer can't react", http.MethodPost, p + "/reactions", viewerID, react, 403},
		{"Viewer can't un-react", http.MethodDelete, p + "/reactions", viewerID, nil, 403},
		{"Viewer can't react to a reply", http.MethodPost, c, viewerID, react, 403},
		{"Owner reacts anywhere", http.MethodPost, p + "/reactions", ownerID, react, 200},
		{"learner reacts to a reply", http.MethodPost, c, learner13ID, react, 200},
		{"learner removes a reply reaction", http.MethodDelete, c, learner13ID, nil, 200},
		{"other-cohort learner can't remove a reply reaction", http.MethodDelete, c, learner14ID, nil, 403},
		{"missing post is 404", http.MethodGet, "/board/posts/" + primitive.NewObjectID().Hex(), ownerID, nil, 404},
	}
	for _, tc := range cases {
		if got := call(t, app, tc.method, tc.path, tc.as, tc.body); got != tc.want {
			t.Errorf("%s: got %d, want %d", tc.name, got, tc.want)
		}
	}
	if !repo.pulledComment {
		t.Error("removing a reply reaction should pull the caller's reaction")
	}
}

func TestOutOfScopeTAIsBlockedEverywhere(t *testing.T) {
	commentID := primitive.NewObjectID()
	repo := &fakeBoardRepo{post: domain.Post{ID: primitive.NewObjectID(), Cohort: 14, Comments: []domain.Comment{{ID: commentID}}}}
	app := boardTestApp(repo)
	p := "/board/posts/" + repo.post.ID.Hex()
	c := p + "/comments/" + commentID.Hex() + "/reactions"
	for _, tc := range []struct{ method, path string }{
		{http.MethodGet, p},
		{http.MethodPost, p + "/comments"},
		{http.MethodPost, p + "/reactions"},
		{http.MethodDelete, p + "/reactions"},
		{http.MethodPost, c},
		{http.MethodDelete, c},
	} {
		if got := call(t, app, tc.method, tc.path, ta13ID, map[string]string{"content": "x", "reaction": "👍"}); got != 403 {
			t.Errorf("%s %s: got %d, want 403", tc.method, tc.path, got)
		}
	}
	if len(repo.comments) != 0 || repo.postReactions != 0 || repo.commentReacts != 0 {
		t.Error("nothing should be written for an out-of-scope TA")
	}
}

func TestStaffReplyTakesThePostsCohort(t *testing.T) {
	repo := &fakeBoardRepo{post: domain.Post{ID: primitive.NewObjectID(), Cohort: 13}}
	app := boardTestApp(repo)
	if got := call(t, app, http.MethodPost, "/board/posts/"+repo.post.ID.Hex()+"/comments", ta13ID, map[string]string{"content": "nice"}); got != 200 {
		t.Fatalf("got %d", got)
	}
	if len(repo.comments) != 1 || repo.comments[0].Cohort != 13 {
		t.Errorf("staff reply should be stored with the post's cohort 13, got %+v", repo.comments)
	}
}

func TestReplyReactionReplacesTheCallersOld(t *testing.T) {
	commentID := primitive.NewObjectID()
	repo := &fakeBoardRepo{post: domain.Post{ID: primitive.NewObjectID(), Cohort: 13, Comments: []domain.Comment{{ID: commentID}}}}
	app := boardTestApp(repo)
	path := "/board/posts/" + repo.post.ID.Hex() + "/comments/" + commentID.Hex() + "/reactions"
	if got := call(t, app, http.MethodPost, path, learner13ID, map[string]string{"reaction": "👍"}); got != 200 {
		t.Fatalf("got %d", got)
	}
	if !repo.pulledComment || repo.commentReacts != 1 {
		t.Errorf("should pull the old reaction then add one, pulled=%v added=%d", repo.pulledComment, repo.commentReacts)
	}
}

func TestReplyReactionOnMissingCommentIs404(t *testing.T) {
	repo := &fakeBoardRepo{post: domain.Post{ID: primitive.NewObjectID(), Cohort: 13}}
	app := boardTestApp(repo)
	path := "/board/posts/" + repo.post.ID.Hex() + "/comments/" + primitive.NewObjectID().Hex() + "/reactions"
	if got := call(t, app, http.MethodPost, path, learner13ID, map[string]string{"reaction": "👍"}); got != 404 {
		t.Errorf("got %d, want 404", got)
	}
}

func TestStaffReplyIsNamedAndReactable(t *testing.T) {
	repo := &fakeBoardRepo{post: domain.Post{ID: primitive.NewObjectID(), Cohort: 13}}
	app := boardTestApp(repo)
	if got := call(t, app, http.MethodPost, "/board/posts/"+repo.post.ID.Hex()+"/comments", ta13ID, map[string]string{"content": "nice"}); got != 200 {
		t.Fatalf("got %d", got)
	}
	c := repo.comments[0]
	if c.ZoomName != "Kan" {
		t.Errorf("a staff member without a Zoom name should reply as their first name, got %q", c.ZoomName)
	}
	if c.Reactions == nil {
		t.Error("a new reply must start with an empty reactions list (null can't take a reaction)")
	}
}

func TestBoardCohortForReadRefusesALearnerWithNoCohort(t *testing.T) {
	if _, ok := boardCohortForRead(boardActor{ownCohort: 0}, 0); ok {
		t.Error("a learner with no cohort must not get every cohort's posts")
	}
}
