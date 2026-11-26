package handler

import "testing"

func TestUserListFieldsDropHeavyFields(t *testing.T) {
	for _, f := range []string{"reflections", "password", "profile_comments", "profile_reactions", "plant_reactions", "care_energy_log", "fertilizer_log", "gift_boxes", "owned_cosmetic_ids", "new_cosmetic_ids"} {
		if v, ok := userListFields[f]; !ok || v != 0 {
			t.Errorf("%s not excluded from GET /users", f)
		}
	}
	for _, keep := range []string{"badges", "social_links", "bio", "skills", "avatar"} {
		if _, excluded := userListFields[keep]; excluded {
			t.Errorf("%s must stay in GET /users", keep)
		}
	}
}
