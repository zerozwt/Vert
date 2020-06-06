package main

import (
	"bytes"
	"crypto/tls"
	"crypto/x509"
	"errors"
	"io/ioutil"
	"net/http"
	"sync"
	"time"

	"golang.org/x/crypto/acme/autocert"
	"golang.org/x/crypto/ocsp"
)

type certInfo struct {
	AutoCert bool

	SSLKey  string
	SSLCert string
}

type ocspInfo struct {
	data   []byte
	expire time.Time
}

type ocspManager struct {
	lock  sync.Mutex
	cache map[string]ocspInfo
}

type staticCertManager struct {
	lock  sync.Mutex
	cache map[[2]string]*tls.Certificate // key: [cert_file, key_file]
}

var gCertInfo map[string]certInfo = make(map[string]certInfo)
var gCertManager *autocert.Manager

var gOCSPManager *ocspManager = &ocspManager{
	cache: make(map[string]ocspInfo),
}

var gStaticCertManager *staticCertManager = &staticCertManager{
	cache: make(map[[2]string]*tls.Certificate),
}

func initCertManager() {
	gCertManager = &autocert.Manager{
		Prompt: autocert.AcceptTOS,
		Email:  gConf.Base.TlsEmail,
	}
	if len(gConf.Base.CertCache) != 0 {
		gCertManager.Cache = autocert.DirCache(gConf.Base.CertCache)
	}
}

func setCertInfo(name string, info certInfo) error {
	if _, ok := gCertInfo[name]; ok {
		return errors.New("Duplicate cert info: " + name)
	}
	gCertInfo[name] = info
	return nil
}

func certManagerWhitelist(hosts ...string) {
	gCertManager.HostPolicy = autocert.HostWhitelist(hosts...)
}

func getCert(hello *tls.ClientHelloInfo) (*tls.Certificate, error) {
	name := hello.ServerName

	info, ok := gCertInfo[name]
	if !ok {
		ERROR_LOG("No certificates available for %s", name)
		return nil, errors.New("No certificates available for " + name)
	}

	if !info.AutoCert {
		ret, err := gStaticCertManager.Get(info.SSLCert, info.SSLKey)
		if err != nil {
			ERROR_LOG("Load cert from file for %s failed: %v", name, err)
			return nil, err
		}
		return ret, nil
	}

	cert, err := gCertManager.GetCertificate(hello)
	if err != nil {
		ERROR_LOG("Auto cert for %s failed: %v", name, err)
		return nil, err
	}

	ocsp, err := getOCSP(name, cert)
	if err != nil {
		ERROR_LOG("Get OCSP for %s failed: %v", name, err)
	} else {
		cert.OCSPStaple = ocsp
	}

	return cert, nil
}

func getOCSP(name string, cert *tls.Certificate) ([]byte, error) {
	if cache, ok := gOCSPManager.Get(name); ok {
		return cache, nil
	}

	x509Cert := cert.Leaf
	ocspServer := x509Cert.OCSPServer[0]
	x509Issuer, err := x509.ParseCertificate(cert.Certificate[1])
	if err != nil {
		ERROR_LOG("x509 parse certificate for %s failed: %v", name, err)
		return nil, err
	}

	ocspRequest, err := ocsp.CreateRequest(x509Cert, x509Issuer, nil)
	if err != nil {
		ERROR_LOG("create ocsp request for %s failed: %v", name, err)
		return nil, err
	}

	ocspRequestReader := bytes.NewReader(ocspRequest)
	httpResponse, err := http.Post(ocspServer, "application/ocsp-request", ocspRequestReader)
	if err != nil {
		ERROR_LOG("request ocsp for %s failed: %v", name, err)
		return nil, err
	}
	defer httpResponse.Body.Close()

	ocspResponseBytes, err := ioutil.ReadAll(httpResponse.Body)
	if err != nil {
		ERROR_LOG("read ocsp response for %s failed: %v", name, err)
		return nil, err
	}

	ocspResponse, err := ocsp.ParseResponse(ocspResponseBytes, x509Issuer)
	if err != nil {
		ERROR_LOG("parse ocsp response for %s failed: %v", name, err)
		return nil, err
	}

	if ocspResponse.Status != ocsp.Good {
		ERROR_LOG("ocsp status for %s is not good (value=%d) retuen ocsp info but do not cache", name, ocspResponse.Status)
		return ocspResponseBytes, nil
	}

	gOCSPManager.Set(name, ocspResponseBytes, ocspResponse.NextUpdate)

	return ocspResponseBytes, nil
}

func (self *ocspManager) Get(name string) ([]byte, bool) {
	self.lock.Lock()
	defer self.lock.Unlock()

	info, ok := self.cache[name]
	if !ok {
		return nil, false
	}

	if time.Now().After(info.expire) {
		delete(self.cache, name)
		return nil, false
	}

	return info.data, true
}

func (self *ocspManager) Set(name string, data []byte, expire time.Time) {
	self.lock.Lock()
	defer self.lock.Unlock()

	self.cache[name] = ocspInfo{
		data:   data,
		expire: expire,
	}
}

func (self *staticCertManager) Get(certFile, keyFile string) (*tls.Certificate, error) {
	self.lock.Lock()
	defer self.lock.Unlock()

	cache_key := [2]string{certFile, keyFile}
	cache, ok := self.cache[cache_key]

	if ok {
		tmNow := time.Now()
		if tmNow.Before(cache.Leaf.NotBefore) || tmNow.Add(time.Hour*24).After(cache.Leaf.NotAfter) {
			ok = false
		}
	}

	if ok {
		return cache, nil
	}

	cert, err := tls.LoadX509KeyPair(certFile, keyFile)
	if err != nil {
		return nil, err
	}

	cert.Leaf, err = x509.ParseCertificate(cert.Certificate[0])
	if err != nil {
		return nil, err
	}

	self.cache[cache_key] = &cert
	return &cert, nil
}
