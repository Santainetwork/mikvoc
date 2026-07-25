package httpapi

import (
	"net/http"

	"mikvoc/internal/database"
)

// HandleRouters renders the router session selection page.
func (a *App) HandleRouters(w http.ResponseWriter, r *http.Request) {
	var routers []database.Router
	if a.Routers != nil {
		list, _ := a.Routers.List()
		routers = make([]database.Router, len(list))
		for i, rt := range list {
			routers[i] = database.Router{
				ID:              rt.ID,
				Name:            rt.Name,
				IP:              rt.IP,
				Port:            rt.Port,
				Username:        rt.Username,
				Password:        rt.Password,
				SortOrder:       rt.SortOrder,
				VoucherTemplate: rt.VoucherTemplate,
			}
		}
	} else {
		routers, _ = database.GetRouters()
	}
	activeID := sessionRouterID(r)

	a.render(w, r, "routers.html", TemplateData{
		Title:      "Sesi Router — MikVoc",
		ActiveMenu: "routers",
		ActiveIdx:  activeID,
		Data:       routers,
	})
}
