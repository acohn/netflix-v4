package main

//This simple DNS client/server filters out ipv6 addresses from Netflix's domains.

import (
	"flag"
	"github.com/miekg/dns"
	"log"
	"os"
	"os/signal"
	"sync"
	"syscall"
)

//use one and only one dns client object
var resolver = &dns.Client{}

var wg = sync.WaitGroup{}

//command-line flags
var (
	debugMode        bool
	listenPort       uint
	upstreamResolver string
)

//set up flags
func init() {
	flag.BoolVar(&debugMode, "d", false, "Print debug info")
	flag.UintVar(&listenPort, "p", 5353, "Listen on this TCP/UDP port")
	flag.StringVar(&upstreamResolver, "r", "8.8.8.8:53", "DNS resolver to proxy")
}

func main() {
	flag.Parse()

	serveMux := dns.NewServeMux()
	serveMux.HandleFunc(".", handleNormalRequest) //if for some reason we get a non-Netflix dns request, handle it
	serveMux.HandleFunc("netflix.com.", handleNetflixRequest)
	serveMux.HandleFunc("nflxvideo.com.", handleNetflixRequest)
	serveMux.HandleFunc("nflxext.com.", handleNetflixRequest)
	serveMux.HandleFunc("nflximg.com.", handleNetflixRequest)

	servers := make(chan *dns.Server, 2)

	wg.Add(1)
	go func() {
		defer wg.Done()
		server := &dns.Server{Addr: ":" + strconv.Itoa(port), Net: "udp", Handler: serveMux}
		servers <- server
		if err := server.ListenAndServe(); err != nil {
			log.Printf("UDP listener err: %v", err)
		}
	}()

	wg.Add(1)
	go func() {
		defer wg.Done()
		server := &dns.Server{Addr: ":" + strconv.Itoa(port), Net: "tcp", Handler: serveMux}
		servers <- server
		if err := server.ListenAndServe(); err != nil {
			log.Printf("TCP listener err: %v", err)
		}
	}()

	sig := make(chan os.Signal)
	signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)
	_ = <-sig
	for i := 0; i < 2; i++ {
		server := <-servers
		server.Shutdown()
	}
	wg.Wait()

}

func handleNormalRequest(writer dns.ResponseWriter, req *dns.Msg) {
	if debugMode {
		log.Printf("received non-Netflix query: %v", req)
	}
	response := fetchProxiedResult(req)
	if debugMode {
		log.Printf("sending response to non-Netflix query: %v", response)
	}
	writer.WriteMsg(response)
}

func handleNetflixRequest(writer dns.ResponseWriter, req *dns.Msg) {
	if debugMode {
		log.Printf("received Netflix query: %v", req)
	}
	response := fetchProxiedResult(req)
	response.Answer = filterRRSet(response.Answer)
	response.Extra = filterRRSet(response.Extra)
	if debugMode {
		log.Printf("sending response to Netflix query: %v", response)
	}
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
	nestedResponse, _, err := resolver.Exchange(nestedQuery, upstreamResolver)
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
