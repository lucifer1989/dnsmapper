package main

import (
	"flag"
	"github.com/abh/dns"
	"log"
	"os"
	"os/signal"
	"strings"
)

var VERSION = "0.0.4"

var (
	flagdomain   = flag.String("domain", "example.com", "base domain for the dnsmapper")
	flagip       = flag.String("ip", "127.0.0.1", "set the IP address")
	flagdnsport  = flag.String("dnsport", "53", "Set the DNS port")
	flaghttpport = flag.String("httpport", "80", "Set the HTTP port")
	flaglog      = flag.Bool("log", false, "be more verbose")

	flagPrimaryNs = flag.String("ns", "ns.example.com", "nameserver names (comma separated)")
)

var baseLength int

var primaryNsList []string

func getUuidFromDomain(name string) string {
	lx := dns.SplitDomainName(name)
	if len(lx) <= baseLength {
		return ""
	}
	ql := lx[0 : len(lx)-baseLength]
	return strings.ToLower(strings.Join(ql, "."))
}

func setup() {
	baseLength = dns.CountLabel(*flagdomain)

	primaryNsList = strings.Split(*flagPrimaryNs, ",")

	log.Println("Listening for requests to", *flagdomain)
}

func main() {
	flag.Parse()
	log.Printf("Starting dnsmapper %s\n", VERSION)

	setup()

	dns.HandleFunc(*flagdomain, setupServerFunc())

	redisConnect()

	go httpHandler()
	go listenAndServeDNS(*flagip + ":" + *flagdnsport)

	terminate := make(chan os.Signal)
	signal.Notify(terminate, os.Interrupt)

	<-terminate
	log.Printf("dnsmapper: signal received, stopping")

}
