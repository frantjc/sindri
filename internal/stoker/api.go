package stoker

import (
	"cmp"
	"context"
	_ "embed"
	"encoding/json"
	"fmt"
	"net/http"
	"path"
	"strconv"
	"time"

	"github.com/frantjc/sindri/internal/httputil"
	"github.com/frantjc/sindri/internal/logutil"
	"github.com/frantjc/sindri/steamapp"
	"github.com/go-chi/chi"
	"github.com/go-chi/chi/middleware"
	"github.com/go-openapi/spec"
	"github.com/google/uuid"
	swagger "github.com/swaggo/http-swagger/v2"
	"github.com/timewasted/go-accept-headers"
)

type APIHandlerOpts struct {
	Path        string
	Fallback    http.Handler
	Swagger     bool
	SwaggerOpts []func(*spec.Swagger)
}

type APIHandlerOpt interface {
	Apply(*APIHandlerOpts)
}

func (o *APIHandlerOpts) Apply(opts *APIHandlerOpts) {
	if o != nil {
		if opts != nil {
			if o.Path != "" {
				opts.Path = path.Join("/", o.Path)
			}
			if o.Fallback != nil {
				opts.Fallback = o.Fallback
			}
			if o.Swagger {
				opts.Swagger = true
			}
			if len(o.SwaggerOpts) > 0 {
				if opts.SwaggerOpts == nil {
					opts.SwaggerOpts = make([]func(*spec.Swagger), 0)
				}
				opts.SwaggerOpts = append(opts.SwaggerOpts, o.SwaggerOpts...)
			}
		}
	}
}

func newAPIHandlerOpts(opts ...APIHandlerOpt) *APIHandlerOpts {
	o := &APIHandlerOpts{Fallback: http.NotFoundHandler()}

	for _, opt := range opts {
		opt.Apply(o)
	}

	return o
}

type UpsertOpts struct {
	Branch string
}

type UpsertOpt interface {
	ApplyToUpsert(*UpsertOpts)
}

func (o *UpsertOpts) ApplyToUpsert(opts *UpsertOpts) {
	if o != nil {
		if o.Branch != "" {
			opts.Branch = o.Branch
		}
	}
}

type GetOpts struct {
	Branch string
}

type GetOpt interface {
	ApplyToGet(*GetOpts)
}

func (o *GetOpts) ApplyToGet(opts *GetOpts) {
	if o != nil {
		if o.Branch != "" {
			opts.Branch = o.Branch
		}
	}
}

type ListOpts struct {
	Continue string
	Limit    int64
}

type ListOpt interface {
	ApplyToList(*ListOpts)
}

func (o *ListOpts) ApplyToList(opts *ListOpts) {
	if o != nil {
		if o.Continue != "" {
			opts.Continue = o.Continue
		}
		if o.Limit > 0 {
			opts.Limit = o.Limit
		}
	}
}

type Database interface {
	Upsert(context.Context, int, *SteamappDetail, ...UpsertOpt) error
	Get(context.Context, int, ...GetOpt) (*Steamapp, error)
	List(context.Context, ...ListOpt) ([]SteamappSummary, string, error)
}

type handler struct {
	Database Database
	Path     string
}

var (
	//go:embed swagger.json
	swaggerJSON []byte
)

func NewAPIHandler(database Database, opts ...APIHandlerOpt) http.Handler {
	var (
		o = newAPIHandlerOpts(opts...)
		h = &handler{Database: database, Path: o.Path}
		r = chi.NewRouter()
	)

	r.Use(middleware.RealIP)
	r.Use(func(h http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			log := logutil.SloggerFrom(r.Context()).With("request", uuid.NewString())
			log.Info(r.URL.Path, "method", r.Method)
			h.ServeHTTP(w, r.WithContext(logutil.SloggerInto(r.Context(), log)))
		})
	})

	var (
		s = &spec.Swagger{}
		p = path.Join("/", o.Path)
	)

	if err := json.Unmarshal(swaggerJSON, s); err != nil {
		panic(err)
	}

	s.BasePath = p

	for _, opt := range o.SwaggerOpts {
		opt(s)
	}

	r.Route(p, func(r chi.Router) {
		if o.Swagger {
			r.Get("/", http.RedirectHandler(path.Join(p, "/index.html"), http.StatusMovedPermanently).ServeHTTP)

			r.Get("/doc.json", func(w http.ResponseWriter, r *http.Request) {
				_ = respondJSON(w, r, s)
			})

			r.Get("/*", swagger.Handler())
		}

		r.Post(
			fmt.Sprintf("/steamapps/{%s}", appIDParam),
			handleErr(h.upsertSteamapp),
		)
		r.Post(
			fmt.Sprintf("/steamapps/{%s}/{%s}", appIDParam, branchParam),
			handleErr(h.upsertSteamapp),
		)
		r.Put(
			fmt.Sprintf("/steamapps/{%s}", appIDParam),
			handleErr(h.upsertSteamapp),
		)
		r.Put(
			fmt.Sprintf("/steamapps/{%s}/{%s}", appIDParam, branchParam),
			handleErr(h.upsertSteamapp),
		)
		r.Get(
			fmt.Sprintf("/steamapps/{%s}", appIDParam),
			handleErr(h.getSteamapp),
		)
		r.Get(
			fmt.Sprintf("/steamapps/{%s}/{%s}", appIDParam, branchParam),
			handleErr(h.getSteamapp),
		)
		r.Get(
			"/steamapps",
			handleErr(h.getSteamapps),
		)
	})

	r.NotFound(o.Fallback.ServeHTTP)

	return r
}

type Error struct {
	Message string `json:"error,omitempty"`
}

func handleErr(handler func(w http.ResponseWriter, r *http.Request) error) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if err := handler(w, r); err != nil {
			log := logutil.SloggerFrom(r.Context())

			log.Error("handling request", "err", err.Error())

			if nErr := negotiate(w, r, "application/json"); nErr != nil {
				log.Error("negotiating JSON error response", "err", err.Error())

				http.Error(w, err.Error(), httputil.HTTPStatusCode(err))
				return
			}

			w.WriteHeader(httputil.HTTPStatusCode(err))
			_ = respondJSON(w, r, &Error{Message: err.Error()})
		}
	}
}

func negotiate(w http.ResponseWriter, r *http.Request, contentType string) error {
	if _, err := accept.Negotiate(r.Header.Get("Accept"), contentType); err != nil {
		w.Header().Set("Accept", contentType)
		return httputil.NewHTTPStatusCodeError(err, http.StatusUnsupportedMediaType)
	}

	w.Header().Set("Vary", "Accept")

	w.Header().Set("Content-Type", contentType)

	return nil
}

func wantsPretty(r *http.Request) bool {
	pretty, _ := strconv.ParseBool(r.URL.Query().Get("pretty"))
	return pretty
}

func respondJSON(w http.ResponseWriter, r *http.Request, a any) error {
	log := logutil.SloggerFrom(r.Context())

	if err := negotiate(w, r, "application/json"); err != nil {
		log.Error("negotiating JSON response", "err", err.Error())

		return err
	}

	enc := json.NewEncoder(w)
	if wantsPretty(r) {
		enc.SetIndent("", "  ")
	}

	return enc.Encode(a)
}

const (
	appIDParam  = "appID"
	branchParam = "branch"
)

// @Summary	Get the details for a specific Steamapp ID
// @Produce	json
// @Param		appID	path		int		true	"Steamapp ID"
// @Param		branch	path		string	false	"Steamapp branch (default public)"
// @Success	200		{object}	Steamapp
// @Failure	400		{object}	Error
// @Failure	415		{object}	Error
// @Failure	500		{object}	Error
// @Router		/steamapps/{appID} [get]
// @Router		/steamapps/{appID}/{branch} [get]
func (h *handler) getSteamapp(w http.ResponseWriter, r *http.Request) error {
	var (
		steamappID = chi.URLParam(r, appIDParam)
		log        = logutil.SloggerFrom(r.Context()).With(appIDParam, steamappID)
	)
	r = r.WithContext(logutil.SloggerInto(r.Context(), log))

	parsedSteamappAppID, err := strconv.Atoi(steamappID)
	if err != nil {
		return httputil.NewHTTPStatusCodeError(fmt.Errorf("parse Steamapp ID: %w", err), http.StatusBadRequest)
	}

	steamapp, err := h.Database.Get(r.Context(), parsedSteamappAppID, &GetOpts{
		Branch: chi.URLParam(r, branchParam),
	})
	if err != nil {
		return fmt.Errorf("get Steamapp: %w", err)
	}

	return respondJSON(w, r, steamapp)
}

// @Summary	List known Steamapps
// @Produce	json
// @Param		continue	query		string	false	"Continue token"
// @Param		limit		query		int		false	"Maximum number of Steamapps to return (default 10)"
// @Success	200			{array}		SteamappSummary
// @Failure	415			{object}	Error
// @Failure	500			{object}	Error
// @Router		/steamapps [get]
func (h *handler) getSteamapps(w http.ResponseWriter, r *http.Request) error {
	var (
		_        = logutil.SloggerFrom(r.Context())
		limit, _ = strconv.Atoi(r.URL.Query().Get("limit"))
	)
	if limit <= 0 {
		limit = 10
	}

	steamapps, token, err := h.Database.List(r.Context(), &ListOpts{
		Continue: r.URL.Query().Get("continue"),
		Limit:    int64(limit),
	})
	if err != nil {
		return err
	}

	if token != "" {
		w.Header().Set("X-Continue-Token", token)
		w.Header().Set("Link", fmt.Sprintf("%s?continue=%s", path.Join(h.Path, "steamapps"), token))
	}

	return respondJSON(w, r, steamapps)
}

type SteamappResources struct {
	CPU    string `json:"cpu,omitempty"`
	Memory string `json:"memory,omitempty"`
}

type Steamapp struct {
	SteamappSummary
	SteamappDetail
}

type SteamappDetail struct {
	Ports        []SteamappPort    `json:"ports,omitempty"`
	Resources    SteamappResources `json:"resources,omitempty"`
	Volumes      []SteamappVolume  `json:"volumes,omitempty"`
	BaseImageRef string            `json:"base_image,omitempty"`
	AptPkgs      []string          `json:"apt_packages,omitempty"`
	BetaPassword string            `json:"beta_password,omitempty"`
	LaunchType   string            `json:"launch_type,omitempty"`
	PlatformType string            `json:"platform_type,omitempty"`
	Execs        []string          `json:"execs,omitempty"`
	Entrypoint   []string          `json:"entrypoint,omitempty"`
	Cmd          []string          `json:"cmd,omitempty"`
}

type SteamappPort struct {
	Port      int32    `json:"port"`
	Protocols []string `json:"protocols,omitempty"`
}

type SteamappVolume struct {
	Path string `json:"path"`
}

type SteamappSummary struct {
	AppID   int       `json:"app_id,omitempty"`
	Name    string    `json:"name,omitempty"`
	Branch  string    `json:"branch,omitempty"`
	IconURL string    `json:"icon_url,omitempty"`
	Created time.Time `json:"date_created,omitempty"`
	Locked  bool      `json:"locked,omitempty"`
}

// @Summary	Create or update the details of a specific Steamapp ID
// @Accept		application/json
// @Produce	json
// @Param		appID	path	int				true	"Steamapp ID"
// @Param		branch	path	string			false	"Steamapp branch (default public)"
// @Param		request	body	SteamappDetail	true	"Steamapp detail"
// @Success	202
// @Failure	400	{object}	Error
// @Failure	415	{object}	Error
// @Failure	500	{object}	Error
// @Router		/steamapps/{appID} [post]
// @Router		/steamapps/{appID}/{branch} [post]
// @Router		/steamapps/{appID} [put]
// @Router		/steamapps/{appID}/{branch} [put]
func (h *handler) upsertSteamapp(w http.ResponseWriter, r *http.Request) error {
	var (
		steamappID = chi.URLParam(r, appIDParam)
		log        = logutil.SloggerFrom(r.Context()).With(appIDParam, steamappID)
	)
	r = r.WithContext(logutil.SloggerInto(r.Context(), log))

	parsedSteamappAppID, err := strconv.Atoi(steamappID)
	if err != nil {
		return httputil.NewHTTPStatusCodeError(err, http.StatusBadRequest)
	}

	if err := steamapp.ValidateAppID(parsedSteamappAppID); err != nil {
		return httputil.NewHTTPStatusCodeError(err, http.StatusBadRequest)
	}

	var (
		detail = &SteamappDetail{}
		branch = cmp.Or(chi.URLParam(r, branchParam), steamapp.DefaultBranchName)
	)
	if err := json.NewDecoder(r.Body).Decode(detail); err != nil {
		return httputil.NewHTTPStatusCodeError(err, http.StatusBadRequest)
	}

	if detail.BetaPassword != "" {
		if branch == steamapp.DefaultBranchName {
			return httputil.NewHTTPStatusCodeError(fmt.Errorf("cannot set beta_password on branch %q", branch), http.StatusBadRequest)
		}
	}

	if err := h.Database.Upsert(r.Context(), parsedSteamappAppID, detail, &UpsertOpts{Branch: branch}); err != nil {
		return fmt.Errorf("upsert Steamapp: %w", err)
	}

	w.WriteHeader(http.StatusAccepted)

	return nil
}
