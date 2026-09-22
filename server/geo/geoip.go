package geo

import (
	"hash/fnv"
	"log"
	"net"
	"path/filepath"
	"runtime"
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
	Province       string `maxminddb:"province"`
	ProvinceShort  string `maxminddb:"provinceShort"`
	City           string `maxminddb:"city"`
	CityShort      string `maxminddb:"cityShort"`
	Districts      string `maxminddb:"districts"`
	DistrictsShort string `maxminddb:"districtsShort"`
	ISP            string `maxminddb:"isp"`
	Net            string `maxminddb:"net"`
}

var chinaProvinceCoords = map[string][2]float64{
	"北京":  {116.4074, 39.9042},
	"天津":  {117.2008, 39.0842},
	"上海":  {121.4737, 31.2304},
	"重庆":  {106.5516, 29.5630},
	"河北":  {114.5305, 38.0374},
	"山西":  {112.5627, 37.8735},
	"辽宁":  {123.4294, 41.8357},
	"吉林":  {125.3268, 43.8962},
	"黑龙江": {126.6617, 45.7423},
	"江苏":  {118.7969, 32.0603},
	"浙江":  {120.1551, 30.2741},
	"安徽":  {117.2849, 31.8612},
	"福建":  {119.2965, 26.0745},
	"江西":  {115.8163, 28.6366},
	"山东":  {117.0208, 36.6686},
	"河南":  {113.7534, 34.7659},
	"湖北":  {114.3419, 30.5465},
	"湖南":  {112.9838, 28.1124},
	"广东":  {113.2644, 23.1291},
	"海南":  {110.3492, 20.0174},
	"四川":  {104.0759, 30.6517},
	"贵州":  {106.7072, 26.5982},
	"云南":  {102.7100, 25.0458},
	"陕西":  {108.9398, 34.3416},
	"甘肃":  {103.8263, 36.0594},
	"青海":  {101.7801, 36.6209},
	"台湾":  {121.5091, 25.0443},
	"内蒙古": {111.7656, 40.8175},
	"广西":  {108.3275, 22.8155},
	"西藏":  {91.1172, 29.6469},
	"宁夏":  {106.2588, 38.4713},
	"新疆":  {87.6278, 43.7928},
	"香港":  {114.1694, 22.3193},
	"澳门":  {113.5439, 22.1987},
}

// chinaCityCoords 中国主要地级市及重点区县坐标（精度优先）
var chinaCityCoords = map[string][2]float64{
	// 福建省
	"泉州": {118.6757, 24.8741},
	"晋江": {118.5755, 24.8197},
	"石狮": {118.6480, 24.7298},
	"南安": {118.3855, 24.9606},
	"惠安": {118.7967, 25.0321},
	"安溪": {118.1871, 25.0563},
	"永春": {118.2936, 25.3218},
	"德化": {118.2415, 25.4924},
	"厦门": {118.1039, 24.4892},
	"思明": {118.0827, 24.4454},
	"湖里": {118.1468, 24.5132},
	"集美": {118.0972, 24.5758},
	"海沧": {118.0329, 24.4846},
	"同安": {118.1520, 24.7307},
	"翔安": {118.2479, 24.6183},
	"福州": {119.2965, 26.0745},
	"鼓楼": {119.3005, 26.0822},
	"台江": {119.3096, 26.0583},
	"仓山": {119.3209, 26.0389},
	"马尾": {119.4585, 25.9918},
	"晋安": {119.3283, 26.0789},
	"长乐": {119.5231, 25.9614},
	"福清": {119.3849, 25.7208},
	"闽侯": {119.1419, 26.1504},
	"漳州": {117.6534, 24.5129},
	"龙文": {117.7107, 24.5097},
	"芗城": {117.6543, 24.5108},
	"龙海": {117.8181, 24.4465},
	"莆田": {119.0078, 25.4540},
	"城厢": {118.9944, 25.4454},
	"荔城": {119.0142, 25.4337},
	"南平": {118.1785, 26.6421},
	"建阳": {118.1215, 27.3323},
	"武夷山": {118.0367, 27.7554},
	"三明": {117.6387, 26.2635},
	"沙县": {117.7925, 26.3965},
	"龙岩": {117.0298, 25.0763},
	"新罗": {117.0371, 25.0864},
	"宁德": {119.5479, 26.6656},
	"蕉城": {119.5269, 26.6592},
	"福安": {119.6496, 27.0867},
	"福鼎": {120.2166, 27.3243},
	"平潭": {119.7912, 25.5037},

	// 广东重点城市
	"广州": {113.2644, 23.1291},
	"深圳": {114.0579, 22.5431},
	"东莞": {113.7518, 23.0207},
	"佛山": {113.1214, 23.0215},
	"珠海": {113.5767, 22.2707},
	"惠州": {114.4162, 23.1118},
	"中山": {113.3928, 22.5176},
	"汕头": {116.6820, 23.3541},
	"江门": {113.0815, 22.5787},
	"湛江": {110.3594, 21.2707},
	"肇庆": {112.4651, 23.0472},
	"清远": {113.0560, 23.6818},
	"潮州": {116.6226, 23.6569},
	"揭阳": {116.3728, 23.5497},

	// 浙江重点城市
	"杭州": {120.1551, 30.2741},
	"宁波": {121.5497, 29.8683},
	"温州": {120.6994, 27.9943},
	"嘉兴": {120.7555, 30.7461},
	"湖州": {120.0868, 30.8943},
	"绍兴": {120.5802, 30.0303},
	"金华": {119.6474, 29.0791},
	"义乌": {120.0744, 29.3056},
	"衢州": {118.8759, 28.9405},
	"舟山": {122.2072, 29.9853},
	"台州": {121.4208, 28.6564},
	"丽水": {119.9228, 28.4677},

	// 江苏重点城市
	"南京": {118.7969, 32.0603},
	"苏州": {120.5853, 31.2990},
	"昆山": {120.9807, 31.3846},
	"无锡": {120.3119, 31.4912},
	"常州": {119.9741, 31.8112},
	"镇江": {119.4258, 32.1878},
	"南通": {120.8943, 31.9802},
	"泰州": {119.9229, 32.4555},
	"扬州": {119.4129, 32.3942},
	"盐城": {120.1636, 33.3474},
	"淮安": {119.0153, 33.6104},
	"宿迁": {118.2752, 33.9630},
	"徐州": {117.1848, 34.2618},
	"连云港": {119.2216, 34.5967},

	// 山东重点城市
	"济南": {117.0208, 36.6686},
	"青岛": {120.3826, 36.0671},
	"淄博": {118.0591, 36.8047},
	"烟台": {121.4479, 37.4638},
	"潍坊": {119.1618, 36.7068},
	"济宁": {116.5872, 35.4154},
	"威海": {122.1205, 37.5131},
	"临沂": {118.3564, 35.1047},

	// 四川 / 湖北 / 湖南 / 河南 / 陕西 / 安徽 / 江西
	"成都": {104.0665, 30.5728},
	"绵阳": {104.7417, 31.4640},
	"武汉": {114.3055, 30.5928},
	"襄阳": {112.1441, 32.0424},
	"宜昌": {111.2865, 30.6919},
	"长沙": {112.9388, 28.2282},
	"株洲": {113.1340, 27.8274},
	"湘潭": {112.9441, 27.8297},
	"郑州": {113.6254, 34.7466},
	"洛阳": {112.4540, 34.6197},
	"西安": {108.9402, 34.3416},
	"咸阳": {108.7051, 34.3291},
	"合肥": {117.2272, 31.8206},
	"芜湖": {118.3765, 31.3263},
	"南昌": {115.8579, 28.6829},
	"赣州": {114.9359, 25.8318},
	"九江": {115.9928, 29.7120},

	// 其他重点省会与直辖市
	"北京": {116.4074, 39.9042},
	"上海": {121.4737, 31.2304},
	"天津": {117.2008, 39.0842},
	"重庆": {106.5516, 29.5630},
	"石家庄": {114.5149, 38.0428},
	"太原": {112.5489, 37.8706},
	"沈阳": {123.4315, 41.8057},
	"大连": {121.6147, 38.9140},
	"长春": {125.3235, 43.8171},
	"哈尔滨": {126.5350, 45.8038},
	"海口": {110.3283, 20.0319},
	"三亚": {109.5119, 18.2528},
	"贵阳": {106.6302, 26.6477},
	"昆明": {102.8329, 24.8801},
	"兰州": {103.8343, 36.0611},
	"西宁": {101.7782, 36.6171},
	"银川": {106.2309, 38.4872},
	"乌鲁木齐": {87.6177, 43.7928},
	"呼和浩特": {111.7492, 40.8426},
	"南宁": {108.3665, 22.8172},
	"拉萨": {91.1409, 29.6456},
	"香港": {114.1694, 22.3193},
	"澳门": {113.5439, 22.1987},
	"台北": {121.5654, 25.0330},
	"高雄": {120.3014, 22.6273},
}

// worldCountryCoords 国际主要国家及地区的地理中心经纬度 [lng, lat]
var worldCountryCoords = map[string][2]float64{
	"美国":   {-98.5795, 39.8283},
	"美利坚":  {-98.5795, 39.8283},
	"日本":   {139.6917, 35.6895},
	"韩国":   {126.9780, 37.5665},
	"新加坡":  {103.8198, 1.3521},
	"英国":   {-0.1278, 51.5074},
	"德国":   {10.4515, 51.1657},
	"法国":   {2.3522, 48.8566},
	"荷兰":   {4.9041, 52.3676},
	"俄罗斯":  {37.6173, 55.7558},
	"加拿大":  {-106.3468, 56.1304},
	"澳大利亚": {133.7751, -25.2744},
	"澳洲":   {133.7751, -25.2744},
	"印度":   {78.9629, 20.5937},
	"巴西":   {-51.9253, -14.2350},
	"越南":   {108.2772, 14.0583},
	"泰国":   {100.9925, 15.8700},
	"马来西亚": {101.9758, 4.2105},
	"印尼":   {113.9213, -0.7893},
	"印度尼西亚": {113.9213, -0.7893},
	"菲律宾":  {121.7740, 12.8797},
	"瑞士":   {8.2275, 46.8182},
	"瑞典":   {18.0686, 59.3293},
	"意大利":  {12.5674, 41.8719},
	"西班牙":  {-3.7038, 40.4168},
	"爱尔兰":  {-6.2603, 53.3498},
	"阿联酋":  {55.2708, 25.2048},
	"土耳其":  {35.2433, 38.9637},
	"波兰":   {19.1451, 51.9194},
	"乌克兰":  {31.1656, 48.3794},
	"芬兰":   {24.9384, 60.1699},
	"挪威":   {10.7522, 59.9139},
	"墨西哥":  {-102.5528, 23.6345},
	"新西兰":  {174.8860, -40.9006},
	"南非":   {22.9375, -30.5595},
}

// matchCityCoord 优先匹配中国地级市/区县经纬度
func matchCityCoord(cityName, districtName string) ([2]float64, bool) {
	// 1. 优先匹配区县 (如 晋江、石狮、思明)
	if districtName != "" {
		for name, coord := range chinaCityCoords {
			if strings.Contains(districtName, name) {
				return coord, true
			}
		}
	}
	// 2. 匹配地级市 (如 泉州、厦门、福州)
	if cityName != "" {
		for name, coord := range chinaCityCoords {
			if strings.Contains(cityName, name) {
				return coord, true
			}
		}
	}
	return [2]float64{}, false
}

func getProvinceCoord(province, city string) ([2]float64, bool) {
	for name, coord := range chinaProvinceCoords {
		if strings.Contains(province, name) || strings.Contains(city, name) {
			return coord, true
		}
	}
	return [2]float64{}, false
}

// matchWorldCountryCoord 匹配国际国家经纬度
func matchWorldCountryCoord(countryName string) ([2]float64, bool) {
	for name, coord := range worldCountryCoords {
		if strings.Contains(countryName, name) {
			return coord, true
		}
	}
	return [2]float64{}, false
}

func isDirectMunicipality(name string) bool {
	return strings.Contains(name, "北京") || strings.Contains(name, "上海") ||
		strings.Contains(name, "天津") || strings.Contains(name, "重庆")
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
		if runtime.GOOS == "windows" {
			dbDir = "./data/geoip"
		} else {
			dbDir = "/data/geoip"
		}
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

	// 1. 优先检索预定义的知名权威 IP 范围（Cloudflare, Google, GitHub, CDN 等）
	for _, rule := range gs.rules {
		if rule.cidr.Contains(parsedIP) {
			gs.cacheMu.Lock()
			if len(gs.cache) < 20000 {
				gs.cache[ipStr] = rule.loc
			}
			gs.cacheMu.Unlock()
			return rule.loc
		}
	}

	// 2. 本地 MMDB 检索
	gs.mu.RLock()
	hasCity := gs.cityReader != nil
	hasCN := gs.cnReader != nil
	hasASN := gs.asnReader != nil
	cityR := gs.cityReader
	cnR := gs.cnReader
	asnR := gs.asnReader
	gs.mu.RUnlock()

	if hasCity || hasCN || hasASN {
		loc, ok := gs.lookupMMDB(parsedIP, cityR, cnR, asnR, hasCity, hasCN, hasASN)
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

func (gs *GeoService) lookupMMDB(ip net.IP, cityR *geoip2.Reader, cnR *maxminddb.Reader, asnR *geoip2.Reader, hasCity, hasCN, hasASN bool) (GeoLocation, bool) {
	var countryName, regionName, cityName, ispName string
	var lat, lng float64
	var isChina bool
	var hasPreciseCityCoord bool

	// 1. 优先检索 GeoCN 高精度中国 IP 库（彻底覆盖省市区与运营商）
	var cnRecord GeoCNRecord
	var hasCNData bool
	if hasCN && cnR != nil {
		if err := cnR.Lookup(ip, &cnRecord); err == nil {
			if cnRecord.Province != "" || cnRecord.ISP != "" || cnRecord.City != "" {
				hasCNData = true
				isChina = true
				countryName = "中国"
				if cnRecord.ISP != "" {
					ispName = cnRecord.ISP
				}
			}
		}
	}

	// 2. 检索 GeoLite2-City 国际库（获取经纬度与国际归属）
	if hasCity && cityR != nil {
		if cityRec, err := cityR.City(ip); err == nil && cityRec != nil {
			if countryName == "" {
				countryName = cityRec.Country.Names["zh-CN"]
				if countryName == "" {
					countryName = cityRec.Country.Names["en"]
				}
				if countryName == "" {
					countryName = cityRec.RegisteredCountry.Names["zh-CN"]
					if countryName == "" {
						countryName = cityRec.RegisteredCountry.Names["en"]
					}
				}
			}
			if cityRec.Country.IsoCode == "CN" || cityRec.RegisteredCountry.IsoCode == "CN" {
				isChina = true
			}

			if len(cityRec.Subdivisions) > 0 && regionName == "" {
				regionName = cityRec.Subdivisions[0].Names["zh-CN"]
				if regionName == "" {
					regionName = cityRec.Subdivisions[0].Names["en"]
				}
			}

			if cityName == "" {
				cityName = cityRec.City.Names["zh-CN"]
				if cityName == "" {
					cityName = cityRec.City.Names["en"]
				}
			}

			if cityRec.Location.Latitude != 0 || cityRec.Location.Longitude != 0 {
				lat = cityRec.Location.Latitude
				lng = cityRec.Location.Longitude
			}
		}
	}

	// 3. 检索 GeoLite2-ASN (补充未知运营商)
	if ispName == "" && hasASN && asnR != nil {
		if asnRec, err := asnR.ASN(ip); err == nil && asnRec != nil {
			ispName = asnRec.AutonomousSystemOrganization
		}
	}

	// 4. 特殊涉华区域处理
	if countryName == "香港" || countryName == "澳门" || countryName == "台湾" {
		countryName = "中国" + countryName
		isChina = true
	}

	// 5. 中国区域深度整合与格式化（彻底消除“中国·中国”与跨省错位）
	if isChina {
		countryName = "中国"

		if hasCNData {
			if cnRecord.Province != "" {
				regionName = cnRecord.Province
			}

			if cnRecord.City != "" {
				if cnRecord.Districts != "" && cnRecord.Districts != cnRecord.City {
					cityName = cnRecord.City + "·" + cnRecord.Districts
				} else {
					cityName = cnRecord.City
				}
			} else if cnRecord.Districts != "" {
				cityName = cnRecord.Districts
			} else if cnRecord.Province != "" {
				cityName = cnRecord.Province
			}

			// 优先使用 GeoCN 的地级市/区县匹配精确坐标（彻底纠正 GeoLite2 把福建泉州/晋江错投到武汉或国家中心的问题）
			if cityCoord, found := matchCityCoord(cnRecord.City, cnRecord.Districts); found {
				lng = cityCoord[0]
				lat = cityCoord[1]
				hasPreciseCityCoord = true
			}
		}

		// 直辖市智能处理（避免“北京·北京”或“北京·中国”）
		if isDirectMunicipality(regionName) {
			cityName = regionName
		}

		// 彻底杜绝 cityName 变成 "中国"
		if cityName == "中国" || cityName == "" {
			if regionName != "" && regionName != "中国" {
				cityName = regionName
			} else {
				cityName = "骨干网节点"
			}
		}

		// 经纬度校验与兜底：若无精准地级市坐标，自动通过省份锚定其省中心经纬度
		if !hasPreciseCityCoord && (lat == 0 && lng == 0 || hasCNData) {
			// 如果 GeoCN 有明确省份，且之前坐标未精准匹配城市，优先采用 GeoCN 省份坐标，杜绝漂移到外省
			if coord, found := getProvinceCoord(regionName, cityName); found {
				// 若原本 GeoLite2 给出的坐标偏离过远（例如识别为福建但坐标给到了湖北/北京），以省份为准
				lng = coord[0]
				lat = coord[1]
			} else if lat == 0 && lng == 0 {
				lat = 39.9042
				lng = 116.4074
			}
		}
	} else {
		// 外网节点
		if countryName == "" {
			countryName = "境外节点"
		}

		// 规范外网城市名，杜绝出现“美国·美国”这种国名与城市名重复
		if cityName == "" || cityName == countryName {
			if regionName != "" && regionName != countryName {
				cityName = regionName
			} else if strings.Contains(ispName, "Cloudflare") {
				cityName = "Anycast 节点"
			} else if ispName != "" && ispName != "骨干网节点" {
				cityName = "境外节点"
			} else {
				cityName = "国际骨干网"
			}
		}

		// 外网节点经纬度兜底：绝对不能漂移回中国（如上海 121.47, 31.23）
		if lat == 0 && lng == 0 {
			if coord, found := matchWorldCountryCoord(countryName); found {
				lng = coord[0]
				lat = coord[1]
			} else {
				// 国际未知节点使用国际枢纽轮询散列
				h := fnv.New32a()
				h.Write([]byte(countryName + cityName + ispName))
				idx := 9 + int(h.Sum32())%(len(fallbackHubs)-9)
				hub := fallbackHubs[idx]
				lat = hub.Latitude
				lng = hub.Longitude
			}
		}
	}

	if ispName == "" {
		ispName = "骨干网节点"
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
