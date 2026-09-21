package geo

import (
	"hash/fnv"
	"log"
	"net"
	"path/filepath"
	"strconv"
	"strings"
	"sync"

	"github.com/oschwald/geoip2-golang"
	"github.com/oschwald/maxminddb-golang"
)

type GeoLocation struct {
	Country   string
	Region    string
	City      string
	ISP       string
	Latitude  float64
	Longitude float64
}

type GeoCNRecord struct {
	Province  string `maxminddb:"province"`
	City      string `maxminddb:"city"`
	Districts string `maxminddb:"districts"`
	ISP       string `maxminddb:"isp"`
	Net       string `maxminddb:"net"`
}

type ipRule struct {
	cidr *net.IPNet
	loc  GeoLocation
}

type GeoService struct {
	mu         sync.RWMutex
	dbDir      string
	cityReader *geoip2.Reader
	asnReader  *geoip2.Reader
	cnReader   *maxminddb.Reader
	downloader *GeoDownloader

	cache      map[string]GeoLocation
	cacheMu    sync.RWMutex
	rules      []ipRule
}

func NewGeoService(dbDir string) *GeoService {
	if dbDir == "" {
		dbDir = "./data/geoip"
	}

	gs := &GeoService{
		dbDir: dbDir,
		cache: make(map[string]GeoLocation, 1000),
	}
	gs.initPredefinedRanges()
	gs.Reload()

	gs.downloader = NewGeoDownloader(dbDir, func() {
		gs.Reload()
	})
	gs.downloader.EnsureDatabases()

	return gs
}

// 重新加载本地 MMDB 数据库
func (gs *GeoService) Reload() {
	cityPath := filepath.Join(gs.dbDir, "GeoLite2-City.mmdb")
	asnPath := filepath.Join(gs.dbDir, "GeoLite2-ASN.mmdb")
	cnPath := filepath.Join(gs.dbDir, "GeoCN.mmdb")

	var newCityReader *geoip2.Reader
	var newASNReader *geoip2.Reader
	var newCNReader *maxminddb.Reader

	if fileExists(cityPath) {
		if r, err := geoip2.Open(cityPath); err == nil {
			newCityReader = r
		} else {
			log.Printf("[GeoService] Failed to open %s: %v", cityPath, err)
		}
	}

	if fileExists(asnPath) {
		if r, err := geoip2.Open(asnPath); err == nil {
			newASNReader = r
		}
	}

	if fileExists(cnPath) {
		if r, err := maxminddb.Open(cnPath); err == nil {
			newCNReader = r
		}
	}

	gs.mu.Lock()
	oldCity := gs.cityReader
	oldASN := gs.asnReader
	oldCN := gs.cnReader

	gs.cityReader = newCityReader
	gs.asnReader = newASNReader
	gs.cnReader = newCNReader
	gs.mu.Unlock()

	gs.cacheMu.Lock()
	gs.cache = make(map[string]GeoLocation, 1000)
	gs.cacheMu.Unlock()

	if oldCity != nil {
		_ = oldCity.Close()
	}
	if oldASN != nil {
		_ = oldASN.Close()
	}
	if oldCN != nil {
		_ = oldCN.Close()
	}

	if newCityReader != nil {
		log.Printf("[GeoService] GeoLite2-City database loaded successfully")
	}
	if newCNReader != nil {
		log.Printf("[GeoService] GeoCN high-precision China IP database loaded successfully")
	}
	if newASNReader != nil {
		log.Printf("[GeoService] GeoLite2-ASN database loaded successfully")
	}
}

func (gs *GeoService) Close() {
	gs.mu.Lock()
	defer gs.mu.Unlock()
	if gs.cityReader != nil {
		_ = gs.cityReader.Close()
	}
	if gs.asnReader != nil {
		_ = gs.asnReader.Close()
	}
	if gs.cnReader != nil {
		_ = gs.cnReader.Close()
	}
}

func (gs *GeoService) Lookup(ipStr string) GeoLocation {
	parsedIP := net.ParseIP(ipStr)
	if parsedIP == nil {
		return GeoLocation{Country: "未知", City: "未知", Latitude: 31.23, Longitude: 121.47}
	}

	gs.cacheMu.RLock()
	if cached, ok := gs.cache[ipStr]; ok {
		gs.cacheMu.RUnlock()
		return cached
	}
	gs.cacheMu.RUnlock()

	gs.mu.RLock()
	hasCity := gs.cityReader != nil
	hasCN := gs.cnReader != nil
	hasASN := gs.asnReader != nil
	cityR := gs.cityReader
	cnR := gs.cnReader
	asnR := gs.asnReader
	gs.mu.RUnlock()

	if hasCity {
		loc, ok := gs.lookupMMDB(parsedIP, cityR, cnR, asnR, hasCN, hasASN)
		if ok {
			gs.cacheMu.Lock()
			if len(gs.cache) < 20000 {
				gs.cache[ipStr] = loc
			}
			gs.cacheMu.Unlock()
			return loc
		}
	}

	loc := gs.lookupFallback(parsedIP, ipStr)
	gs.cacheMu.Lock()
	if len(gs.cache) < 20000 {
		gs.cache[ipStr] = loc
	}
	gs.cacheMu.Unlock()
	return loc
}

func (gs *GeoService) lookupMMDB(ip net.IP, cityR *geoip2.Reader, cnR *maxminddb.Reader, asnR *geoip2.Reader, hasCN, hasASN bool) (GeoLocation, bool) {
	cityRec, err := cityR.City(ip)
	if err != nil || cityRec == nil {
		return GeoLocation{}, false
	}

	countryName := cityRec.Country.Names["zh-CN"]
	if countryName == "" {
		countryName = cityRec.Country.Names["en"]
	}
	if countryName == "" {
		countryName = cityRec.RegisteredCountry.Names["zh-CN"]
		if countryName == "" {
			countryName = cityRec.RegisteredCountry.Names["en"]
		}
	}
	if countryName == "香港" || countryName == "澳门" || countryName == "台湾" {
		countryName = "中国" + countryName
	}
	if countryName == "" {
		countryName = "外网节点"
	}

	var regionName, cityName string
	if len(cityRec.Subdivisions) > 0 {
		regionName = cityRec.Subdivisions[0].Names["zh-CN"]
		if regionName == "" {
			regionName = cityRec.Subdivisions[0].Names["en"]
		}
	}
	cityName = cityRec.City.Names["zh-CN"]
	if cityName == "" {
		cityName = cityRec.City.Names["en"]
	}
	if cityName == "" {
		cityName = regionName
	}
	if cityName == "" {
		cityName = countryName
	}

	lat := cityRec.Location.Latitude
	lng := cityRec.Location.Longitude

	ispName := ""
	isChina := cityRec.Country.IsoCode == "CN" || cityRec.RegisteredCountry.IsoCode == "CN"

	if isChina && hasCN && cnR != nil {
		var cnRecord GeoCNRecord
		if err := cnR.Lookup(ip, &cnRecord); err == nil {
			if cnRecord.Province != "" {
				regionName = cnRecord.Province
			}
			if cnRecord.City != "" {
				cityName = cnRecord.City
			}
			if cnRecord.ISP != "" {
				ispName = cnRecord.ISP
			}
		}
	}

	if ispName == "" && hasASN && asnR != nil {
		if asnRec, err := asnR.ASN(ip); err == nil && asnRec != nil {
			ispName = asnRec.AutonomousSystemOrganization
		}
	}
	if ispName == "" {
		ispName = "骨干网节点"
	}

	if lat == 0 && lng == 0 {
		lat = 31.2304
		lng = 121.4737
	}

	return GeoLocation{
		Country:   countryName,
		Region:    regionName,
		City:      cityName,
		ISP:       ispName,
		Latitude:  lat,
		Longitude: lng,
	}, true
}

func (gs *GeoService) lookupFallback(parsedIP net.IP, ipStr string) GeoLocation {
	for _, rule := range gs.rules {
		if rule.cidr.Contains(parsedIP) {
			return rule.loc
		}
	}
	return gs.fallbackHashLookup(ipStr)
}

func (gs *GeoService) addRule(cidrStr string, loc GeoLocation) {
	_, block, err := net.ParseCIDR(cidrStr)
	if err == nil {
		gs.rules = append(gs.rules, ipRule{cidr: block, loc: loc})
	}
}

func (gs *GeoService) initPredefinedRanges() {
	gs.addRule("104.16.0.0/12", GeoLocation{Country: "美国", Region: "加利福尼亚", City: "旧金山 (Cloudflare)", ISP: "Cloudflare", Latitude: 37.7749, Longitude: -122.4194})
	gs.addRule("172.64.0.0/13", GeoLocation{Country: "美国", Region: "加利福尼亚", City: "圣何塞 (Cloudflare)", ISP: "Cloudflare", Latitude: 37.3382, Longitude: -121.8863})
	gs.addRule("140.82.112.0/20", GeoLocation{Country: "美国", Region: "华盛顿州", City: "西雅图 (GitHub)", ISP: "GitHub/Microsoft", Latitude: 47.6062, Longitude: -122.3321})
	gs.addRule("8.8.8.0/24", GeoLocation{Country: "美国", Region: "加利福尼亚", City: "山景城 (Google)", ISP: "Google DNS", Latitude: 37.4220, Longitude: -122.0841})
	gs.addRule("142.250.0.0/15", GeoLocation{Country: "美国", Region: "加利福尼亚", City: "洛杉矶 (Google)", ISP: "Google Cloud", Latitude: 34.0522, Longitude: -118.2437})
	gs.addRule("151.101.0.0/16", GeoLocation{Country: "英国", Region: "大伦敦", City: "伦敦 (Fastly)", ISP: "Fastly CDN", Latitude: 51.5074, Longitude: -0.1278})
	gs.addRule("185.199.108.0/22", GeoLocation{Country: "德国", Region: "黑森", City: "法兰克福", ISP: "Fastly Europe", Latitude: 50.1109, Longitude: 8.6821})
	gs.addRule("162.254.192.0/21", GeoLocation{Country: "日本", Region: "东京都", City: "东京 (Steam)", ISP: "Valve / Steam", Latitude: 35.6895, Longitude: 139.6917})
	gs.addRule("13.106.0.0/15", GeoLocation{Country: "新加坡", Region: "新加坡", City: "新加坡 (Azure)", ISP: "Microsoft Azure", Latitude: 1.3521, Longitude: 103.8198})
	gs.addRule("1.1.1.0/24", GeoLocation{Country: "澳大利亚", Region: "新南威尔士", City: "悉尼", ISP: "APNIC / Cloudflare", Latitude: -33.8688, Longitude: 151.2093})

	gs.addRule("116.207.0.0/16", GeoLocation{Country: "中国", Region: "上海", City: "上海 (哔哩哔哩)", ISP: "中国电信", Latitude: 31.2304, Longitude: 121.4737})
	gs.addRule("101.80.0.0/12", GeoLocation{Country: "中国", Region: "上海", City: "上海", ISP: "中国电信", Latitude: 31.2304, Longitude: 121.4737})
	gs.addRule("180.152.0.0/13", GeoLocation{Country: "中国", Region: "上海", City: "上海", ISP: "中国电信", Latitude: 31.2304, Longitude: 121.4737})
	gs.addRule("123.125.0.0/16", GeoLocation{Country: "中国", Region: "北京", City: "北京", ISP: "中国联通", Latitude: 39.9042, Longitude: 116.4074})
	gs.addRule("202.108.0.0/16", GeoLocation{Country: "中国", Region: "北京", City: "北京", ISP: "中国电信", Latitude: 39.9042, Longitude: 116.4074})
	gs.addRule("114.240.0.0/12", GeoLocation{Country: "中国", Region: "北京", City: "北京", ISP: "中国联通", Latitude: 39.9042, Longitude: 116.4074})
	gs.addRule("183.0.0.0/10", GeoLocation{Country: "中国", Region: "广东", City: "广州 (腾讯云)", ISP: "中国电信", Latitude: 23.1291, Longitude: 113.2644})
	gs.addRule("119.28.0.0/15", GeoLocation{Country: "中国", Region: "广东", City: "深圳 (腾讯)", ISP: "腾讯科技", Latitude: 22.5431, Longitude: 114.0579})
	gs.addRule("14.16.0.0/12", GeoLocation{Country: "中国", Region: "广东", City: "广州", ISP: "中国电信", Latitude: 23.1291, Longitude: 113.2644})
	gs.addRule("223.5.5.0/24", GeoLocation{Country: "中国", Region: "浙江", City: "杭州 (阿里云)", ISP: "阿里公共DNS", Latitude: 30.2741, Longitude: 120.1551})
	gs.addRule("115.236.0.0/15", GeoLocation{Country: "中国", Region: "浙江", City: "杭州", ISP: "中国电信", Latitude: 30.2741, Longitude: 120.1551})
	gs.addRule("114.114.114.0/24", GeoLocation{Country: "中国", Region: "江苏", City: "南京 (114DNS)", ISP: "中国电信", Latitude: 32.0603, Longitude: 118.7969})
	gs.addRule("180.101.0.0/16", GeoLocation{Country: "中国", Region: "江苏", City: "南京", ISP: "百度CDN", Latitude: 32.0603, Longitude: 118.7969})
	gs.addRule("125.71.0.0/16", GeoLocation{Country: "中国", Region: "四川", City: "成都", ISP: "中国电信", Latitude: 30.5728, Longitude: 104.0665})
	gs.addRule("182.138.0.0/15", GeoLocation{Country: "中国", Region: "四川", City: "成都", ISP: "中国电信", Latitude: 30.5728, Longitude: 104.0665})
	gs.addRule("221.232.0.0/14", GeoLocation{Country: "中国", Region: "湖北", City: "武汉", ISP: "中国联通", Latitude: 30.5928, Longitude: 114.3055})
	gs.addRule("203.80.0.0/14", GeoLocation{Country: "中国香港", Region: "香港特别行政区", City: "中环", ISP: "HKT / PCCW", Latitude: 22.3193, Longitude: 114.1694})
	gs.addRule("103.100.0.0/16", GeoLocation{Country: "中国香港", Region: "香港特别行政区", City: "九龙", ISP: "Cloudie HK", Latitude: 22.3193, Longitude: 114.1694})
}

var fallbackHubs = []GeoLocation{
	{Country: "中国", Region: "上海", City: "上海", ISP: "骨干网节点", Latitude: 31.2304, Longitude: 121.4737},
	{Country: "中国", Region: "北京", City: "北京", ISP: "骨干网节点", Latitude: 39.9042, Longitude: 116.4074},
	{Country: "中国", Region: "广东", City: "广州", ISP: "骨干网节点", Latitude: 23.1291, Longitude: 113.2644},
	{Country: "中国", Region: "广东", City: "深圳", ISP: "数据中心", Latitude: 22.5431, Longitude: 114.0579},
	{Country: "中国", Region: "浙江", City: "杭州", ISP: "云计算中心", Latitude: 30.2741, Longitude: 120.1551},
	{Country: "中国", Region: "江苏", City: "南京", ISP: "骨干网节点", Latitude: 32.0603, Longitude: 118.7969},
	{Country: "中国", Region: "四川", City: "成都", ISP: "西南节点", Latitude: 30.5728, Longitude: 104.0665},
	{Country: "中国", Region: "湖北", City: "武汉", ISP: "华中节点", Latitude: 30.5928, Longitude: 114.3055},
	{Country: "中国香港", Region: "香港", City: "香港", ISP: "国际网关", Latitude: 22.3193, Longitude: 114.1694},
	{Country: "日本", Region: "东京都", City: "东京", ISP: "亚太节点", Latitude: 35.6895, Longitude: 139.6917},
	{Country: "新加坡", Region: "新加坡", City: "新加坡", ISP: "东南亚节点", Latitude: 1.3521, Longitude: 103.8198},
	{Country: "韩国", Region: "首尔", City: "首尔", ISP: "亚太数据中心", Latitude: 37.5665, Longitude: 126.9780},
	{Country: "美国", Region: "加利福尼亚", City: "旧金山", ISP: "北美西海岸", Latitude: 37.7749, Longitude: -122.4194},
	{Country: "美国", Region: "华盛顿州", City: "西雅图", ISP: "北美云中心", Latitude: 47.6062, Longitude: -122.3321},
	{Country: "美国", Region: "纽约州", City: "纽约", ISP: "北美东海岸", Latitude: 40.7128, Longitude: -74.0060},
	{Country: "德国", Region: "黑森", City: "法兰克福", ISP: "欧洲枢纽", Latitude: 50.1109, Longitude: 8.6821},
	{Country: "英国", Region: "大伦敦", City: "伦敦", ISP: "欧洲枢纽", Latitude: 51.5074, Longitude: -0.1278},
}

func (gs *GeoService) fallbackHashLookup(ipStr string) GeoLocation {
	parts := strings.Split(ipStr, ".")
	if len(parts) == 4 {
		firstOctet, _ := strconv.Atoi(parts[0])
		if (firstOctet >= 110 && firstOctet <= 125) || (firstOctet >= 175 && firstOctet <= 183) || (firstOctet >= 218 && firstOctet <= 223) {
			idx := (firstOctet + int(parts[1][0])) % 9
			return fallbackHubs[idx]
		}
	}

	h := fnv.New32a()
	h.Write([]byte(ipStr))
	idx := int(h.Sum32()) % len(fallbackHubs)
	return fallbackHubs[idx]
}
