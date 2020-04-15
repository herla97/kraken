package engine

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/MixinNetwork/mixin/logger"
	"github.com/dimfeld/httptreemux"
	"github.com/gorilla/handlers"
	"github.com/unrolled/render"
)

type R struct {
	router *Router
}

type Call struct {
	Id     string        `json:"id"`
	Method string        `json:"method"`
	Params []interface{} `json:"params"`
}

type Render struct {
	w    http.ResponseWriter
	impl *render.Render
	id   string
}

func (r *Render) RenderData(data interface{}) {
	body := map[string]interface{}{"data": data}
	if r.id != "" {
		body["id"] = r.id
	}
	r.impl.JSON(r.w, http.StatusOK, body)
}

func (r *Render) RenderError(err error) {
	body := map[string]interface{}{"error": err.Error()}
	if r.id != "" {
		body["id"] = r.id
	}
	r.impl.JSON(r.w, http.StatusOK, body)
}

func (impl *R) handle(w http.ResponseWriter, r *http.Request, _ map[string]string) {
	var call Call
	d := json.NewDecoder(r.Body)
	d.UseNumber()
	if err := d.Decode(&call); err != nil {
		render.New().JSON(w, http.StatusBadRequest, map[string]interface{}{"error": err.Error()})
		return
	}
	renderer := &Render{w: w, impl: render.New(), id: call.Id}
	switch call.Method {
	case "list":
		peers, err := impl.router.rpcList(call.Params)
		if err != nil {
			renderer.RenderError(err)
		} else {
			renderer.RenderData(map[string][]string{"peers": peers})
		}
	case "publish":
		answer, err := impl.router.rpcPublish(call.Params)
		if err != nil {
			renderer.RenderError(err)
		} else {
			renderer.RenderData(answer)
		}
	case "trickle":
		err := impl.router.rpcTrickle(call.Params)
		if err != nil {
			renderer.RenderError(err)
		} else {
			renderer.RenderData(map[string]string{})
		}
	case "subscribe":
		answer, err := impl.router.rpcSubscribe(call.Params)
		if err != nil {
			renderer.RenderError(err)
		} else {
			renderer.RenderData(answer)
		}
	default:
		renderer.RenderError(fmt.Errorf("invalid method %s", call.Method))
	}
}

func registerHandlers(router *httptreemux.TreeMux) {
	router.MethodNotAllowedHandler = func(w http.ResponseWriter, r *http.Request, _ map[string]httptreemux.HandlerFunc) {
		render.New().JSON(w, http.StatusNotFound, map[string]interface{}{"error": "not found"})
	}
	router.NotFoundHandler = func(w http.ResponseWriter, r *http.Request) {
		render.New().JSON(w, http.StatusNotFound, map[string]interface{}{"error": "not found"})
	}
	router.PanicHandler = func(w http.ResponseWriter, r *http.Request, rcv interface{}) {
		render.New().JSON(w, http.StatusInternalServerError, map[string]interface{}{"error": "server error"})
	}
}

func handleCORS(handler http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		origin := r.Header.Get("Origin")
		if origin == "" {
			handler.ServeHTTP(w, r)
			return
		}
		w.Header().Set("Access-Control-Allow-Origin", origin)
		w.Header().Add("Access-Control-Allow-Headers", "Content-Type,Authorization,Mixin-Conversation-ID")
		w.Header().Set("Access-Control-Allow-Methods", "OPTIONS,GET,POST,DELETE")
		w.Header().Set("Access-Control-Max-Age", "600")
		if r.Method == "OPTIONS" {
			render.New().JSON(w, http.StatusOK, map[string]interface{}{})
		} else {
			handler.ServeHTTP(w, r)
		}
	})
}

func ServeRPC(engine *Engine, conf *Configuration) error {
	logger.Printf("ServeRPC(:%d)\n", conf.RPC.Port)
	impl := &R{router: NewRouter(engine)}
	router := httptreemux.New()
	router.POST("/", impl.handle)
	registerHandlers(router)
	handler := handleCORS(router)
	handler = handlers.ProxyHeaders(handler)

	server := &http.Server{
		Addr:         fmt.Sprintf(":%d", conf.RPC.Port),
		Handler:      handler,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 10 * time.Second,
		IdleTimeout:  120 * time.Second,
	}
	return server.ListenAndServe()
}
