package main

import (
	"github.com/abh/dns"
	"log"
	"net"
)

func setupSOA() *dns.SOA {

	s := *flagdomain + ". 3600 IN SOA " +
		primaryNsList[0] + " hostmaster " +
		"1 5400 5400 2419200 300"

	rr, err := dns.NewRR(s)

	if err != nil {
		log.Println("SOA Error", err)
		panic("Could not setup SOA")
	}

	return rr.(*dns.SOA)
}

func setupNS() []dns.RR {

	nsList := make([]dns.RR, 0)

	for _, ns := range primaryNsList {
		s := *flagdomain + ". 20800 IN NS " + ns + "."

		rr, err := dns.NewRR(s)

		if err != nil {
			log.Println("NS Error", err)
			panic("Could not setup NS")
		}
		nsList = append(nsList, rr)
	}

	return nsList
}

func getEdnsSubNet(req *dns.Msg) (ip string, rr *dns.OPT, edns *dns.EDNS0_SUBNET) {

	for _, extra := range req.Extra {
		// log.Println("Extra:", extra)
		for _, o := range extra.(*dns.OPT).Option {
			// opt_rr = extra.(*dns.OPT)
			switch e := o.(type) {
			case *dns.EDNS0_NSID:
				// do stuff with e.Nsid
			case *dns.EDNS0_SUBNET:
				// log.Println("Got edns", e.Address, e.Family, e.SourceNetmask, e.SourceScope)
				if e.Address != nil {
					edns = e
					rr = extra.(*dns.OPT)
					ip = e.Address.String()
				}
			}
		}
	}
	return
}

func setupServerFunc() func(dns.ResponseWriter, *dns.Msg) {

	soa := setupSOA()
	ns := setupNS()

	h := &dns.RR_Header{Ttl: 10, Class: dns.ClassINET, Rrtype: dns.TypeA}
	a := &dns.A{Hdr: *h, A: net.ParseIP(*flagip)}

	return func(w dns.ResponseWriter, req *dns.Msg) {

		m := new(dns.Msg)
		m.SetReply(req)
		if e := m.IsEdns0(); e != nil {
			m.SetEdns0(4096, e.Do())
		}
		m.Authoritative = true

		uuid := getUuidFromDomain(req.Question[0].Name)

		qtype := req.Question[0].Qtype

		if qtype == dns.TypeNS && len(uuid) == 0 {
			m.Answer = ns
			w.WriteMsg(m)
			return
		}

		// we only know how to do A records
		if qtype != dns.TypeA {
			m.Ns = []dns.RR{soa}
			w.WriteMsg(m)
			return
		}

		log.Println("uuid", uuid)

		if len(uuid) > 0 {

			ednsIP, extraRr, edns := getEdnsSubNet(req)
			ip, _, _ := net.SplitHostPort(w.RemoteAddr().String())

			log.Println("Setting answer for ip:", ip)
			a.Header().Name = req.Question[0].Name
			m.Answer = []dns.RR{a}

			Redis.SetEx("dns-"+uuid, 10, ip)
			if len(ednsIP) > 0 {
				Redis.SetEx("dnsedns-"+uuid, 10, ednsIP)

				if edns != nil {
					// log.Println("family", edns.Family)
					if edns.Family != 0 {
						edns.SourceScope = 24
						m.Extra = append(m.Extra, extraRr)
					}
				}

			}
		} else {
			// NOERROR
			w.WriteMsg(m)
			return
		}

		if len(m.Answer) == 0 {
			// return NXDOMAIN
			m.SetRcode(req, dns.RcodeNameError)
			m.Authoritative = true
			m.Ns = []dns.RR{soa}
		}

		log.Println("Returning", m)

		w.WriteMsg(m)
		return
	}

}

func listenAndServeDNS(ip string) {

	prots := []string{"udp", "tcp"}

	for _, prot := range prots {
		go func(p string) {
			server := &dns.Server{Addr: ip, Net: p}

			log.Printf("Opening on %s %s", ip, p)
			if err := server.ListenAndServe(); err != nil {
				log.Fatalf("geodns: failed to setup %s %s: %s", ip, p, err)
			}
			log.Fatalf("geodns: ListenAndServe unexpectedly returned")
		}(prot)
	}

}
