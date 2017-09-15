package main

import (
	"flag"
	"github.com/miekg/dns"
	"log"
	"sync"
)

var wg = &sync.WaitGroup{}

var resolver = &dns.Client{}

var (
	debugMode        bool
	listenPort       uint
	upstreamResolver string
)

func init() {
	flag.BoolVar(&debugMode, "d", false, "Print debug info")
	flag.UintVar(&listenPort, "p", 5353, "Listen on this TCP/UDP port")
	flag.StringVar(&upstreamResolver, "r", "8.8.8.8:53", "DNS resolver to proxy")

}

func main() {
	flag.Parse()
	serveMux := dns.NewServeMux()
	serveMux.HandleFunc(".", handleNormalRequest)
	serveMux.HandleFunc("netflix.com.", handleNetflixRequest)
	serveMux.HandleFunc("nflxvideo.com.", handleNetflixRequest)
	serveMux.HandleFunc("nflxext.com.", handleNetflixRequest)
	serveMux.HandleFunc("nflximg.com.", handleNetflixRequest)
	wg.Add(1)
	go func() {
		defer wg.Done()
		server := &dns.Server{Addr: "127.0.0.1:53535", Net: "udp", Handler: serveMux}
		err := server.ListenAndServe()
		if err != nil {
			log.Printf("UDP goroutine err: %v", err)
		}
	}()

	wg.Add(1)
	go func() {
		defer wg.Done()
		server := &dns.Server{Addr: "127.0.0.1:53535", Net: "tcp", Handler: serveMux}
		err := server.ListenAndServe()
		if err != nil {
			log.Printf("TCP goroutine err: %v", err)
		}
	}()

	wg.Wait()

}

func handleNormalRequest(writer dns.ResponseWriter, req *dns.Msg) {
	response := fetchProxiedResult(req)
	writer.WriteMsg(response)
}

func handleNetflixRequest(writer dns.ResponseWriter, req *dns.Msg) {
	response := fetchProxiedResult(req)
	response.Answer = filterRRSet(response.Answer)
	response.Extra = filterRRSet(response.Extra)
	writer.WriteMsg(response)
}

func filterRRSet(rrset []dns.RR) (ret []dns.RR) {
	for _, rr := range rrset {
		header := rr.Header()
		if header.Rrtype != dns.TypeAAAA || header.Class != dns.ClassINET {
			ret = append(ret, rr)
		}
	}
	return
}

func fetchProxiedResult(req *dns.Msg) *dns.Msg {
	nestedQuery := &dns.Msg{}
	nestedQuery.Question = make([]dns.Question, 1)
	nestedQuery.SetEdns0(4096, false)
	nestedQuery.Question[0] = req.Question[0]
	nestedQuery.Id = dns.Id()
	nestedQuery.RecursionDesired = true
	if debugMode {
		log.Printf("sending nested query: %v", nestedQuery)
	}
	nestedResponse, _, err := resolver.Exchange(nestedQuery, "8.8.8.8:53")
	if err != nil {
		log.Print(err)
		response := &dns.Msg{}
		response.SetRcode(req, dns.RcodeServerFailure)
		return response
	}
	if debugMode {
		log.Printf("received nested response: %v", nestedResponse)
	}

	response := &dns.Msg{
		Answer: make([]dns.RR, len(nestedResponse.Answer)),
		Extra:  make([]dns.RR, len(nestedResponse.Extra)),
	}
	response.SetRcode(req, nestedResponse.Rcode)
	response.RecursionAvailable = true
	copy(response.Answer, nestedResponse.Answer)
	copy(response.Extra, nestedResponse.Extra)
	return response
}
