package geoip

import (
	"fmt"
	"net"
	"os"
	"strings"
	"sync"

	"github.com/lionsoul2014/ip2region/binding/golang/xdb"
)

type Location struct {
	Country string
	Region  string
	City    string
	ISP     string
}

func (l Location) String() string {
	parts := make([]string, 0, 3)
	for _, part := range []string{l.Country, l.Region, l.City} {
		part = strings.TrimSpace(part)
		if part == "" || part == "0" {
			continue
		}
		parts = append(parts, part)
	}
	if len(parts) == 0 {
		return ""
	}
	return strings.Join(parts, ", ")
}

type Lookup struct {
	path string
	mu   sync.Mutex
	v4   *xdb.Searcher
}

func New(path string) *Lookup {
	return &Lookup{path: strings.TrimSpace(path)}
}

func (l *Lookup) Lookup(ip string) (Location, error) {
	ip = strings.TrimSpace(ip)
	if ip == "" || ip == "-" {
		return Location{}, nil
	}
	if net.ParseIP(ip) == nil {
		return Location{}, fmt.Errorf("invalid ip %q", ip)
	}
	searcher, err := l.searcher()
	if err != nil {
		return Location{}, err
	}
	if searcher == nil {
		return Location{}, nil
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	region, err := searcher.Search(ip)
	if err != nil {
		return Location{}, err
	}
	return ParseRegion(region), nil
}

func (l *Lookup) searcher() (*xdb.Searcher, error) {
	if l.v4 != nil {
		return l.v4, nil
	}
	if l.path == "" {
		return nil, nil
	}
	if _, err := os.Stat(l.path); err != nil {
		return nil, nil
	}
	searcher, err := xdb.NewWithFileOnly(xdb.IPv4, l.path)
	if err != nil {
		return nil, err
	}
	l.v4 = searcher
	return l.v4, nil
}

func (l *Lookup) Close() error {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.v4 == nil {
		return nil
	}
	l.v4.Close()
	l.v4 = nil
	return nil
}

func ParseRegion(region string) Location {
	parts := strings.Split(region, "|")
	for len(parts) < 5 {
		parts = append(parts, "")
	}
	return Location{
		Country: parts[0],
		Region:  parts[1],
		City:    parts[2],
		ISP:     parts[3],
	}
}
