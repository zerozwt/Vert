package main

import (
	"errors"
	"flag"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v2"
)

type SiteConf struct {
	Type string `yaml:"type"`
	Port int    `yaml:"port"`

	AutoCert bool `yaml:"autocert"`

	SSLKey  string `yaml:"ssl_key"`
	SSLCert string `yaml:"ssl_cert"`

	Rules []map[string][]string `yaml:"rules"`
}

type Conf struct {
	Base struct {
		LogLevel  string `yaml:"log_level"`
		iLogLevel int
		LogFile   string `yaml:"log_file"`

		TlsEmail  string `yaml:"tls_email"`
		CertCache string `yaml:"cert_cache"`
	} `yaml:"base"`
	Sites map[string][]SiteConf `yaml:"sites"`
}

var gConf Conf

func loadConf() error {
	conf_file := ""
	flag.StringVar(&conf_file, "conf", "", "config file")
	flag.Parse()

	if conf_file == "" {
		conf_file, _ = filepath.Abs(filepath.Dir(os.Args[0]))
		conf_file = filepath.Join(conf_file, "conf.yaml")
	}

	conf_data, err := ioutil.ReadFile(conf_file)
	if err != nil {
		return err
	}

	if err = yaml.Unmarshal(conf_data, &gConf); err != nil {
		return err
	}

	log_level := map[string]int{
		"debug": 1,
		"info":  2,
		"error": 3,
	}

	if n, ok := log_level[gConf.Base.LogLevel]; ok {
		gConf.Base.iLogLevel = n
	}

	//check site conf
	sites := make(map[string][]SiteConf)
	for domain, conf_list := range gConf.Sites {
		tmp := make([]SiteConf, 0)
		for _, conf := range conf_list {
			site_type := conf.Type
			port := conf.Port
			if site_type == "" {
				if port == 80 {
					site_type = "http"
				} else if port == 443 {
					site_type = "https"
				} else {
					return errors.New("Cannot automatically determine type of " + domain + ":" + fmt.Sprint(port))
				}
			}
			if port == 0 {
				if site_type == "http" {
					port = 80
				} else if site_type == "https" {
					port = 443
				} else {
					return errors.New("Cannot automatically determine port of " + domain)
				}
			}
			if port < 1 || port > 65535 {
				return errors.New("Invalid port " + fmt.Sprint(port) + " of " + domain)
			}

			tmp = append(tmp, SiteConf{
				Type:     site_type,
				Port:     port,
				AutoCert: conf.AutoCert,
				SSLKey:   conf.SSLKey,
				SSLCert:  conf.SSLCert,
				Rules:    conf.Rules,
			})
		}
		sites[domain] = tmp
	}
	gConf.Sites = sites

	return nil
}
