package sindri

import "net/http"

func Handler() http.Handler {
	mux := http.NewServeMux()

	mux.HandleFunc("GET /v2", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Docker-Distribution-Api-Version", "registry/2.0")
	})

	return mux
}
