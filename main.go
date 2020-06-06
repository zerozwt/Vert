package main

import (
	"errors"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"runtime"
	"syscall"
	"time"

	"github.com/gorilla/mux"
	"github.com/zerozwt/Vert/action"
	"github.com/zerozwt/Vert/env"
)

func waitSignal(done chan bool) {
	ch_sig := make(chan os.Signal, 1)
	signal.Notify(ch_sig, syscall.SIGINT, syscall.SIGTERM)
	<-ch_sig
	INFO_LOG("exit signal recieved")
	close(done)
}

type serverSlot struct {
	port  int
	isTls bool

	router *mux.Router
}

func isTls(scheme string) bool { return scheme == "https" }

func buildServerSlots(sites map[string][]SiteConf) (map[int]*serverSlot, error) {
	ret := make(map[int]*serverSlot)
	ssl_hosts := make([]string, 0)

	for name, site := range sites {
		for _, conf := range site {
			if conf.Type != "http" && conf.Type != "https" {
				return nil, errors.New("Invalid site type " + conf.Type + " for " + name)
			}

			//create slot if not exist
			slot, ok := ret[conf.Port]
			if !ok {
				ret[conf.Port] = &serverSlot{
					port:   conf.Port,
					isTls:  isTls(conf.Type),
					router: mux.NewRouter(),
				}
				slot = ret[conf.Port]
			}

			//check slot type
			if slot.isTls != isTls(conf.Type) {
				return nil, errors.New("Invalid type " + conf.Type + " for " + name + ": incompatible with existing sites")
			}

			//set certificate info
			if slot.isTls {
				info := certInfo{
					AutoCert: conf.AutoCert,
					SSLKey:   conf.SSLKey,
					SSLCert:  conf.SSLCert,
				}
				if err := setCertInfo(name, info); err != nil {
					return nil, err
				}
				if conf.AutoCert {
					ssl_hosts = append(ssl_hosts, name)
				}
			}

			//set router for host
			s := slot.router.Host(name).Subrouter()
			for _, rule := range conf.Rules {
				for path, actions := range rule {
					handler := http.NotFoundHandler()

					for i := len(actions) - 1; i >= 0; i-- {
						var err error
						handler, err = action.ActionHandler(actions[i], handler)
						if err != nil {
							return nil, err
						}
					}

					s.PathPrefix(path).Handler(handler)
				}
			}
		}
	}

	certManagerWhitelist(ssl_hosts...)
	return ret, nil
}

func logHandler(underlying http.Handler) http.Handler {
	return http.HandlerFunc(func(rsp http.ResponseWriter, req *http.Request) {
		INFO_LOG("ACCESS %s %s %s %s %s", req.RemoteAddr, req.Method, req.Host, req.URL.String(), req.Proto)
		underlying.ServeHTTP(rsp, env.WrapRequest(req))
	})
}

func main() {
	if err := loadConf(); err != nil {
		fmt.Println("load config failed: ", err)
		return
	}

	//init base systems
	initLog()
	initCertManager()
	action.SetLogger(Logger{})

	//build server slots
	slots, err := buildServerSlots(gConf.Sites)
	if err != nil {
		fmt.Println("build server slots failed: ", err)
		return
	}

	//set port 80 default slot if not exist
	if _, ok := slots[80]; !ok {
		slots[80] = &serverSlot{
			port:   80,
			isTls:  false,
			router: mux.NewRouter(),
		}
	}
	if slots[80].isTls {
		fmt.Println("Port 80 cannot run HTTPS!")
		return
	}

	tls_config := gCertManager.TLSConfig()
	tls_config.GetCertificate = getCert

	runtime.GOMAXPROCS(runtime.NumCPU())

	//start all server slots
	for port, slot := range slots {
		server := &http.Server{
			Addr:        fmt.Sprintf(":%d", port),
			TLSConfig:   tls_config,
			Handler:     slot.router,
			IdleTimeout: time.Second * 300,
		}

		if port == 80 {
			server.Handler = gCertManager.HTTPHandler(server.Handler)
		}
		server.Handler = logHandler(server.Handler)

		if slot.isTls {
			INFO_LOG("Start HTTPS on port %d ...", port)
			go server.ListenAndServeTLS("", "")
		} else {
			INFO_LOG("Start HTTP on port %d ...", port)
			go server.ListenAndServe()
		}
	}

	ch_done := make(chan bool)
	go waitSignal(ch_done)
	<-ch_done
}
