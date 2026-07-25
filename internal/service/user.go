package service

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"mikvoc/internal/core"
	"mikvoc/internal/routeros"
)

type CommentMultiProfileError struct {
	Comment  string
	Profiles []string
}

func (e *CommentMultiProfileError) Error() string {
	return fmt.Sprintf("comment %q spans multiple profiles: %s", e.Comment, strings.Join(e.Profiles, ", "))
}

func (e *CommentMultiProfileError) Unwrap() error {
	return core.ErrConflict
}

type UserService struct {
	pool *Pool
}

func NewUser(pool *Pool) *UserService {
	return &UserService{pool: pool}
}

func (s *UserService) List(routerID int, filters core.UserFilters) ([]core.User, error) {
	users, _, err := s.ListPage(routerID, filters, 0, 0)
	return users, err
}

func (s *UserService) ListPage(routerID int, filters core.UserFilters, limit, offset int) ([]core.User, int, error) {
	profile := filters.Profile
	if profile == "all" {
		profile = ""
	}
	var users []routeros.HotspotUser
	var err error
	if profile == "" {
		users, err = s.pool.GetUsersCached(routerID, "")
	} else {
		cl, e := s.pool.RequireClient(routerID)
		if e != nil {
			return nil, 0, e
		}
		users, err = cl.GetUsers(profile)
	}
	if err != nil {
		return nil, 0, err
	}
	filtered := filterUsers(users, filters)
	total := len(filtered)
	page := PageSlice(filtered, limit, offset)
	out := make([]core.User, len(page))
	for i, u := range page {
		out[i] = toCoreUser(u)
	}
	return out, total, nil
}

func PageSlice[T any](items []T, limit, offset int) []T {
	if offset < 0 {
		offset = 0
	}
	if offset >= len(items) {
		return items[:0]
	}
	end := len(items)
	if limit > 0 && offset+limit < end {
		end = offset + limit
	}
	return items[offset:end]
}

func ClampLimit(limit, def, max int) int {
	if limit <= 0 {
		return def
	}
	if limit > max {
		return max
	}
	return limit
}

func ClampOffset(offset int) int {
	if offset < 0 {
		return 0
	}
	return offset
}

func (s *UserService) Get(routerID int, id string) (*core.User, error) {
	if u, ok := s.pool.GetUserByID(routerID, id); ok {
		cu := toCoreUser(u)
		return &cu, nil
	}
	users, err := s.pool.GetUsersCached(routerID, "")
	if err != nil {
		return nil, err
	}
	for _, u := range users {
		if u.ID == id {
			cu := toCoreUser(u)
			return &cu, nil
		}
	}
	return nil, core.ErrNotFound
}

func (s *UserService) GetByName(routerID int, name string) (routeros.HotspotUser, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return routeros.HotspotUser{}, core.ErrInvalidInput
	}
	if u, ok := s.pool.GetUserByName(routerID, name); ok {
		return u, nil
	}
	users, err := s.pool.GetUsersCached(routerID, "")
	if err == nil {
		if u, ok := s.pool.GetUserByName(routerID, name); ok {
			return u, nil
		}
		for _, u := range users {
			if u.Name == name {
				return u, nil
			}
		}
	}
	cl, e := s.pool.RequireClient(routerID)
	if e != nil {
		if err != nil {
			return routeros.HotspotUser{}, err
		}
		return routeros.HotspotUser{}, e
	}
	u, e := cl.GetUserByName(name)
	if e != nil {
		return routeros.HotspotUser{}, core.ErrNotFound
	}
	return u, nil
}

func (s *UserService) RemoveByComment(routerID int, comment, profile string) (int, error) {
	comment = strings.TrimSpace(comment)
	profile = strings.TrimSpace(profile)
	if profile == "all" {
		profile = ""
	}
	if comment == "" {
		return 0, core.ErrInvalidInput
	}
	users, err := s.pool.GetUsersCached(routerID, profile)
	if err != nil {
		return 0, err
	}
	candidates := commentRemovalCandidates(users, comment, profile)
	if profile == "" {
		profiles := commentCandidateProfiles(candidates)
		if len(profiles) > 1 {
			return 0, &CommentMultiProfileError{Comment: comment, Profiles: profiles}
		}
	}
	if len(candidates) == 0 {
		return 0, core.ErrNotFound
	}
	cl, err := s.pool.RequireClient(routerID)
	if err != nil {
		return 0, err
	}
	removed := 0
	var firstErr error
	for _, u := range candidates {
		if e := cl.RemoveUser(u.ID); e != nil {
			if firstErr == nil {
				firstErr = e
			}
			continue
		}
		removed++
	}
	if removed > 0 {
		s.pool.InvalidateUsers(routerID)
	}
	return removed, firstErr
}

func commentRemovalCandidates(users []routeros.HotspotUser, comment, profile string) []routeros.HotspotUser {
	comment = strings.TrimSpace(comment)
	profile = strings.TrimSpace(profile)
	if profile == "all" {
		profile = ""
	}
	if comment == "" {
		return nil
	}
	candidates := make([]routeros.HotspotUser, 0)
	for _, u := range users {
		if strings.TrimSpace(u.Comment) != comment {
			continue
		}
		if profile != "" && strings.TrimSpace(u.Profile) != profile {
			continue
		}
		if !routeros.IsUnusedHotspotUser(u) {
			continue
		}
		candidates = append(candidates, u)
	}
	return candidates
}

func commentCandidateProfiles(candidates []routeros.HotspotUser) []string {
	seen := map[string]bool{}
	profiles := make([]string, 0)
	for _, u := range candidates {
		profile := strings.TrimSpace(u.Profile)
		if seen[profile] {
			continue
		}
		seen[profile] = true
		profiles = append(profiles, profile)
	}
	sort.Strings(profiles)
	return profiles
}

func (s *UserService) Update(routerID int, id string, u core.User) error {
	cl, err := s.pool.RequireClient(routerID)
	if err != nil {
		return err
	}
	if err := cl.UpdateUser(id, fromCoreUser(u)); err != nil {
		return err
	}
	s.pool.InvalidateUsers(routerID)
	return nil
}

func (s *UserService) Remove(routerID int, ids []string) (int, error) {
	cl, err := s.pool.RequireClient(routerID)
	if err != nil {
		return 0, err
	}
	removed := 0
	for _, id := range ids {
		if e := cl.RemoveUser(id); e == nil {
			removed++
		}
	}
	if removed > 0 {
		s.pool.InvalidateUsers(routerID)
	}
	return removed, nil
}

func (s *UserService) SetDisabled(routerID int, ids []string, disabled bool) (int, error) {
	cl, err := s.pool.RequireClient(routerID)
	if err != nil {
		return 0, err
	}
	done := 0
	for _, id := range ids {
		if e := cl.SetUserDisabled(id, disabled); e == nil {
			done++
		}
	}
	if done > 0 {
		s.pool.InvalidateUsers(routerID)
	}
	return done, nil
}

func (s *UserService) SetProfile(routerID int, ids []string, profile string) (int, error) {
	cl, err := s.pool.RequireClient(routerID)
	if err != nil {
		return 0, err
	}
	done := 0
	for _, id := range ids {
		if e := cl.UpdateUser(id, routeros.HotspotUser{Profile: profile}); e == nil {
			done++
		}
	}
	if done > 0 {
		s.pool.InvalidateUsers(routerID)
	}
	return done, nil
}

func (s *UserService) Reset(routerID int, id string) error {
	cl, err := s.pool.RequireClient(routerID)
	if err != nil {
		return err
	}
	if err := cl.ResetUser(id); err != nil {
		return err
	}
	s.pool.InvalidateUsers(routerID)
	return nil
}

func (s *UserService) RemoveExpired(routerID int) (int, error) {
	cl, err := s.pool.RequireClient(routerID)
	if err != nil {
		return 0, err
	}
	n, err := cl.RemoveExpiredUsers(cl.ROSVersion())
	if n > 0 {
		s.pool.InvalidateUsers(routerID)
	}
	return n, err
}

func (s *UserService) Active(routerID int, server string) ([]core.ActiveSession, error) {
	cl, err := s.pool.RequireClient(routerID)
	if err != nil {
		return nil, err
	}
	actives, err := cl.GetActiveUsers(server)
	if err != nil {
		return nil, err
	}
	out := make([]core.ActiveSession, len(actives))
	for i, a := range actives {
		out[i] = toCoreActive(a)
	}
	return out, nil
}

func (s *UserService) Kick(routerID int, sessionID string) error {
	cl, err := s.pool.RequireClient(routerID)
	if err != nil {
		return err
	}
	return cl.KickActiveUser(sessionID)
}

func (s *UserService) Add(routerID int, u core.User) error {
	cl, err := s.pool.RequireClient(routerID)
	if err != nil {
		return err
	}
	if err := cl.AddUser(fromCoreUser(u)); err != nil {
		return err
	}
	s.pool.InvalidateUsers(routerID)
	return nil
}

func filterUsers(users []routeros.HotspotUser, filters core.UserFilters) []routeros.HotspotUser {
	filtered := make([]routeros.HotspotUser, 0, len(users))
	selectedOrder := map[string]int{}
	for i, id := range filters.IDs {
		selectedOrder[id] = i
	}
	now := time.Now()
	for _, u := range users {
		if filters.Profile != "" && filters.Profile != "all" && u.Profile != filters.Profile {
			continue
		}
		if filters.Comment != "" && u.Comment != filters.Comment {
			continue
		}
		if filters.Expired && !routeros.IsExpiredHotspotUser(u, now) {
			continue
		}
		if filters.Search != "" && !core.ContainsIgnoreCase(u.Name, filters.Search) &&
			!core.ContainsIgnoreCase(u.Comment, filters.Search) &&
			!core.ContainsIgnoreCase(u.Profile, filters.Search) {
			continue
		}
		if len(filters.IDs) > 0 {
			if _, ok := selectedOrder[u.ID]; !ok {
				continue
			}
		}
		filtered = append(filtered, u)
	}
	if len(filters.IDs) > 0 {
		sort.SliceStable(filtered, func(i, j int) bool {
			return selectedOrder[filtered[i].ID] < selectedOrder[filtered[j].ID]
		})
	}
	return filtered
}
