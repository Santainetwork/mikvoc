package service

import (
	"encoding/csv"
	"fmt"
	"io"
	"strings"

	"mikvoc/internal/core"
	"mikvoc/internal/routeros"
)

type PPPService struct {
	pool *Pool
}

func NewPPP(pool *Pool) *PPPService {
	return &PPPService{pool: pool}
}

func toCorePPPSecret(s routeros.PPPSecret) core.PPPSecret {
	return core.PPPSecret{
		ID: s.ID, Name: s.Name, Password: s.Password, Service: s.Service, Profile: s.Profile,
		LocalAddress: s.LocalAddress, RemoteAddress: s.RemoteAddress, Comment: s.Comment,
		CallerID: s.CallerID, Routes: s.Routes, LimitBytesIn: s.LimitBytesIn, LimitBytesOut: s.LimitBytesOut,
		Disabled: s.Disabled, LastLoggedOut: s.LastLoggedOut,
	}
}

func fromCorePPPSecret(s core.PPPSecret) routeros.PPPSecret {
	return routeros.PPPSecret{
		ID: s.ID, Name: s.Name, Password: s.Password, Service: s.Service, Profile: s.Profile,
		LocalAddress: s.LocalAddress, RemoteAddress: s.RemoteAddress, Comment: s.Comment,
		CallerID: s.CallerID, Routes: s.Routes, LimitBytesIn: s.LimitBytesIn, LimitBytesOut: s.LimitBytesOut,
		Disabled: s.Disabled, LastLoggedOut: s.LastLoggedOut,
	}
}

func toCorePPPProfile(p routeros.PPPProfile) core.PPPProfile {
	return core.PPPProfile{
		ID: p.ID, Name: p.Name, LocalAddress: p.LocalAddress, RemoteAddress: p.RemoteAddress,
		Bridge: p.Bridge, IncomingFilter: p.IncomingFilter, OutgoingFilter: p.OutgoingFilter,
		AddressList: p.AddressList, DNSServer: p.DNSServer, WINSServer: p.WINSServer,
		ChangeTCPMSS: p.ChangeTCPMSS, UseUPnP: p.UseUPnP, RateLimit: p.RateLimit, OnlyOne: p.OnlyOne,
		IsDefault: p.IsDefault,
	}
}

func fromCorePPPProfile(p core.PPPProfile) routeros.PPPProfile {
	return routeros.PPPProfile{
		ID: p.ID, Name: p.Name, LocalAddress: p.LocalAddress, RemoteAddress: p.RemoteAddress,
		Bridge: p.Bridge, IncomingFilter: p.IncomingFilter, OutgoingFilter: p.OutgoingFilter,
		AddressList: p.AddressList, DNSServer: p.DNSServer, WINSServer: p.WINSServer,
		ChangeTCPMSS: p.ChangeTCPMSS, UseUPnP: p.UseUPnP, RateLimit: p.RateLimit, OnlyOne: p.OnlyOne,
		IsDefault: p.IsDefault,
	}
}

func toCorePPPActive(a routeros.PPPActive) core.PPPActive {
	return core.PPPActive{
		ID: a.ID, Name: a.Name, Service: a.Service, CallerID: a.CallerID,
		Address: a.Address, Uptime: a.Uptime, Encoding: a.Encoding, SessionID: a.SessionID,
	}
}

func filterPPPSecrets(secrets []routeros.PPPSecret, f core.PPPFilters) []routeros.PPPSecret {
	profile := strings.TrimSpace(f.Profile)
	if profile == "all" {
		profile = ""
	}
	comment := strings.TrimSpace(f.Comment)
	search := strings.TrimSpace(f.Search)
	idSet := map[string]bool{}
	for _, id := range f.IDs {
		id = strings.TrimSpace(id)
		if id != "" {
			idSet[id] = true
		}
	}
	out := make([]routeros.PPPSecret, 0, len(secrets))
	for _, s := range secrets {
		if profile != "" && s.Profile != profile {
			continue
		}
		if comment != "" && strings.TrimSpace(s.Comment) != comment {
			continue
		}
		if f.Disabled != nil && s.Disabled != *f.Disabled {
			continue
		}
		if len(idSet) > 0 && !idSet[s.ID] {
			continue
		}
		if search != "" {
			if !core.ContainsIgnoreCase(s.Name, search) &&
				!core.ContainsIgnoreCase(s.Comment, search) &&
				!core.ContainsIgnoreCase(s.Profile, search) &&
				!core.ContainsIgnoreCase(s.RemoteAddress, search) {
				continue
			}
		}
		out = append(out, s)
	}
	return out
}

func (s *PPPService) ListSecrets(routerID int, filters core.PPPFilters) ([]core.PPPSecret, error) {
	list, _, err := s.ListSecretsPage(routerID, filters, 0, 0)
	return list, err
}

func (s *PPPService) ListSecretsPage(routerID int, filters core.PPPFilters, limit, offset int) ([]core.PPPSecret, int, error) {
	cl, err := s.pool.RequireClient(routerID)
	if err != nil {
		return nil, 0, err
	}
	profile := filters.Profile
	if profile == "all" {
		profile = ""
	}
	secrets, err := cl.GetPPPSecrets(profile)
	if err != nil {
		return nil, 0, err
	}
	filtered := filterPPPSecrets(secrets, filters)
	total := len(filtered)
	page := PageSlice(filtered, limit, offset)
	out := make([]core.PPPSecret, len(page))
	for i, sec := range page {
		out[i] = toCorePPPSecret(sec)
	}
	return out, total, nil
}

func (s *PPPService) GetSecret(routerID int, id string) (*core.PPPSecret, error) {
	cl, err := s.pool.RequireClient(routerID)
	if err != nil {
		return nil, err
	}
	secrets, err := cl.GetPPPSecrets("")
	if err != nil {
		return nil, err
	}
	for _, sec := range secrets {
		if sec.ID == id {
			c := toCorePPPSecret(sec)
			return &c, nil
		}
	}
	return nil, core.ErrNotFound
}

func (s *PPPService) AddSecret(routerID int, sec core.PPPSecret) error {
	cl, err := s.pool.RequireClient(routerID)
	if err != nil {
		return err
	}
	sec.Name = strings.TrimSpace(sec.Name)
	if sec.Name == "" {
		return core.ErrInvalidInput
	}
	if sec.Password == "" {
		sec.Password = sec.Name
	}
	if sec.Service == "" {
		sec.Service = "pppoe"
	}
	return cl.AddPPPSecret(fromCorePPPSecret(sec))
}

func (s *PPPService) UpdateSecret(routerID int, sec core.PPPSecret) error {
	cl, err := s.pool.RequireClient(routerID)
	if err != nil {
		return err
	}
	if strings.TrimSpace(sec.ID) == "" {
		return core.ErrInvalidInput
	}
	return cl.UpdatePPPSecret(fromCorePPPSecret(sec))
}

func (s *PPPService) RemoveSecrets(routerID int, ids []string) (int, error) {
	cl, err := s.pool.RequireClient(routerID)
	if err != nil {
		return 0, err
	}
	n := 0
	var first error
	for _, id := range ids {
		id = strings.TrimSpace(id)
		if id == "" {
			continue
		}
		if e := cl.RemovePPPSecret(id); e != nil {
			if first == nil {
				first = e
			}
			continue
		}
		n++
	}
	return n, first
}

func (s *PPPService) SetDisabled(routerID int, ids []string, disabled bool) (int, error) {
	cl, err := s.pool.RequireClient(routerID)
	if err != nil {
		return 0, err
	}
	n := 0
	var first error
	for _, id := range ids {
		id = strings.TrimSpace(id)
		if id == "" {
			continue
		}
		if e := cl.SetPPPSecretDisabled(id, disabled); e != nil {
			if first == nil {
				first = e
			}
			continue
		}
		n++
	}
	return n, first
}

func (s *PPPService) Active(routerID int) ([]core.PPPActive, error) {
	cl, err := s.pool.RequireClient(routerID)
	if err != nil {
		return nil, err
	}
	list, err := cl.GetPPPActive()
	if err != nil {
		return nil, err
	}
	out := make([]core.PPPActive, len(list))
	for i, a := range list {
		out[i] = toCorePPPActive(a)
	}
	return out, nil
}

func (s *PPPService) Kick(routerID int, sessionID string) error {
	cl, err := s.pool.RequireClient(routerID)
	if err != nil {
		return err
	}
	sessionID = strings.TrimSpace(sessionID)
	if sessionID == "" {
		return core.ErrInvalidInput
	}
	return cl.KickPPPActive(sessionID)
}

func (s *PPPService) ListProfiles(routerID int) ([]core.PPPProfile, error) {
	cl, err := s.pool.RequireClient(routerID)
	if err != nil {
		return nil, err
	}
	list, err := cl.GetPPPProfiles()
	if err != nil {
		return nil, err
	}
	out := make([]core.PPPProfile, len(list))
	for i, p := range list {
		out[i] = toCorePPPProfile(p)
	}
	return out, nil
}

func (s *PPPService) ListPools(routerID int) ([]string, error) {
	cl, err := s.pool.RequireClient(routerID)
	if err != nil {
		return nil, err
	}
	return cl.GetIPPools()
}

func (s *PPPService) ListBridges(routerID int) ([]string, error) {
	cl, err := s.pool.RequireClient(routerID)
	if err != nil {
		return nil, err
	}
	return cl.GetBridges()
}

func (s *PPPService) CreateProfile(routerID int, p core.PPPProfile) error {
	cl, err := s.pool.RequireClient(routerID)
	if err != nil {
		return err
	}
	p.Name = strings.TrimSpace(p.Name)
	if p.Name == "" {
		return core.ErrInvalidInput
	}
	return cl.CreatePPPProfile(fromCorePPPProfile(p))
}

func (s *PPPService) UpdateProfile(routerID int, p core.PPPProfile) error {
	cl, err := s.pool.RequireClient(routerID)
	if err != nil {
		return err
	}
	if strings.TrimSpace(p.ID) == "" {
		return core.ErrInvalidInput
	}
	return cl.UpdatePPPProfile(fromCorePPPProfile(p))
}

func (s *PPPService) RemoveProfile(routerID int, id string) error {
	cl, err := s.pool.RequireClient(routerID)
	if err != nil {
		return err
	}
	id = strings.TrimSpace(id)
	if id == "" {
		return core.ErrInvalidInput
	}
	return cl.RemovePPPProfile(id)
}

func (s *PPPService) Generate(routerID int, spec core.PPPGenerateSpec) (*core.PPPGenerateBatch, error) {
	cl, err := s.pool.RequireClient(routerID)
	if err != nil {
		return nil, err
	}
	if spec.Qty < 1 || spec.Qty > 500 {
		return nil, core.ErrInvalidInput
	}
	if strings.TrimSpace(spec.Profile) == "" {
		return nil, core.ErrInvalidInput
	}
	if spec.Service == "" {
		spec.Service = "pppoe"
	}
	if spec.Pad < 1 {
		spec.Pad = 3
	}
	created, comment, skipped, err := cl.GeneratePPPSecrets(routeros.PPPGenerateOptions{
		Qty:     spec.Qty,
		Prefix:  spec.Prefix,
		Start:   spec.Start,
		Pad:     spec.Pad,
		Profile: spec.Profile,
		Service: spec.Service,
		Comment: spec.Comment,
	})
	if err != nil && len(created) == 0 {
		return nil, err
	}
	items := make([]core.PPPGenerateResult, len(created))
	for i, pair := range created {
		items[i] = core.PPPGenerateResult{Username: pair[0], Password: pair[1]}
	}
	return &core.PPPGenerateBatch{Comment: comment, Items: items, Skipped: skipped}, err
}

func (s *PPPService) ImportCSV(routerID int, r io.Reader) (int, error) {
	cl, err := s.pool.RequireClient(routerID)
	if err != nil {
		return 0, err
	}
	cr := csv.NewReader(r)
	cr.TrimLeadingSpace = true
	cr.FieldsPerRecord = -1
	rows, err := cr.ReadAll()
	if err != nil {
		return 0, err
	}
	if len(rows) == 0 {
		return 0, core.ErrInvalidInput
	}
	start := 0
	col := map[string]int{}
	header := rows[0]
	looksHeader := false
	for i, h := range header {
		key := strings.ToLower(strings.TrimSpace(h))
		key = strings.TrimPrefix(key, "\ufeff")
		switch key {
		case "name", "username", "user":
			col["name"] = i
			looksHeader = true
		case "password", "pass":
			col["password"] = i
			looksHeader = true
		case "profile":
			col["profile"] = i
			looksHeader = true
		case "service":
			col["service"] = i
			looksHeader = true
		case "comment":
			col["comment"] = i
			looksHeader = true
		case "local-address", "local_address", "local":
			col["local"] = i
			looksHeader = true
		case "remote-address", "remote_address", "remote":
			col["remote"] = i
			looksHeader = true
		}
	}
	if looksHeader {
		start = 1
	} else {
		col["name"] = 0
		if len(header) > 1 {
			col["password"] = 1
		}
		if len(header) > 2 {
			col["profile"] = 2
		}
	}
	get := func(row []string, key string) string {
		i, ok := col[key]
		if !ok || i < 0 || i >= len(row) {
			return ""
		}
		return strings.TrimSpace(row[i])
	}
	n := 0
	var first error
	for _, row := range rows[start:] {
		name := get(row, "name")
		if name == "" {
			continue
		}
		pass := get(row, "password")
		if pass == "" {
			pass = name
		}
		svc := get(row, "service")
		if svc == "" {
			svc = "pppoe"
		}
		sec := routeros.PPPSecret{
			Name:          name,
			Password:      pass,
			Service:       svc,
			Profile:       get(row, "profile"),
			Comment:       get(row, "comment"),
			LocalAddress:  get(row, "local"),
			RemoteAddress: get(row, "remote"),
		}
		if e := cl.AddPPPSecret(sec); e != nil {
			if first == nil {
				first = e
			}
			continue
		}
		n++
	}
	if n == 0 && first != nil {
		return 0, first
	}
	return n, first
}

func WritePPPSecretsCSV(w io.Writer, secrets []core.PPPSecret) error {
	cw := csv.NewWriter(w)
	if err := cw.Write([]string{"name", "password", "service", "profile", "local-address", "remote-address", "comment", "disabled"}); err != nil {
		return err
	}
	for _, s := range secrets {
		dis := "no"
		if s.Disabled {
			dis = "yes"
		}
		if err := cw.Write([]string{s.Name, s.Password, s.Service, s.Profile, s.LocalAddress, s.RemoteAddress, s.Comment, dis}); err != nil {
			return err
		}
	}
	cw.Flush()
	return cw.Error()
}

func PPPGeneratePreview(prefix string, start, pad, qty int) []string {
	if qty < 1 {
		qty = 1
	}
	if qty > 20 {
		qty = 20
	}
	if pad < 1 {
		pad = 3
	}
	out := make([]string, 0, qty)
	for i := 0; i < qty; i++ {
		out = append(out, routeros.FormatPPPSecretName(prefix, start+i, pad))
	}
	return out
}

func FormatPPPBatchSummary(batch *core.PPPGenerateBatch) string {
	if batch == nil {
		return ""
	}
	msg := fmt.Sprintf("Berhasil generate %d PPP secret (comment: %s).", len(batch.Items), batch.Comment)
	if batch.Skipped > 0 {
		msg += fmt.Sprintf(" Skip %d nama yang sudah ada.", batch.Skipped)
	}
	return msg
}
