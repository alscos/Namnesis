package httpserver

import (
	"net"
	"net/http"
	"strings"
)

type cidrAllowlist struct {
	nets []*net.IPNet
}

func newCIDRAllowlist(cidrs []string) (*cidrAllowlist, error) {
	a := &cidrAllowlist{}
	for _, c := range cidrs {
		_, n, err := net.ParseCIDR(strings.TrimSpace(c))
		if err != nil {
			return nil, err
		}
		a.nets = append(a.nets, n)
	}
	return a, nil
}

func (a *cidrAllowlist) middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ipStr := strings.TrimSpace(strings.Split(r.RemoteAddr, ":")[0])
		ip := net.ParseIP(ipStr)
		if ip == nil {
			http.Error(w, "forbidden", http.StatusForbidden)
			return
		}
		for _, n := range a.nets {
			if n.Contains(ip) {
				next.ServeHTTP(w, r)
				return
			}
		}
		http.Error(w, "forbidden", http.StatusForbidden)
	})
}
