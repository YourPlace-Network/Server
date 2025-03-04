package network

import (
	"YourPlace/src/core/security"
	"github.com/ncruces/go-dns"
	"golang.org/x/net/context"
	"log"
	"net"
	"strings"
)

var DoHServers = map[string]string{
	"Quad 9":     "https://dns11.quad9.net/dns-query{?dns}",
	"Google":     "https://dns.google/dns-query{?dns}",
	"Cloudflare": "https://cloudflare-dns.com/dns-query{?dns}",
	"CZ.NIC":     "https://odvr.nic.cz/dns-query{?dns}",
	"Alibaba":    "https://dns.alidns.com/dns-query{?dns}",
}

func ResolveDoH(url string) string {
	if !security.IsValidURL(url) {
		return ""
	}
	return ResolveDoHResolver(DoHServers["Cloudflare"], url)
}
func ResolveDoHResolver(resolverURL string, url string) string {
	resolver, err := dns.NewDoHResolver(resolverURL, dns.DoHCache())
	if err != nil {
		log.Fatal(err)
	}
	ips, err := resolver.LookupIPAddr(context.TODO(), url)
	if err != nil {
		log.Fatal(err)
	}
	builder := strings.Builder{}
	for _, ip := range ips {
		builder.WriteString(ip.String() + "\n")
	}
	return builder.String()
}
func ResolveCNAME(domain string) string {
	cname, err := net.LookupCNAME(domain)
	if err != nil {
		return ""
	}
	return cname
}
