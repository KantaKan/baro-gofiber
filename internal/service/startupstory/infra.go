package startupstory

import (
	"fmt"
	"math"
	"slices"

	"gofiber-baro/internal/domain"
)

const (
	FansPerRequest  = 10
	ServerPrice     = 2000
	ServerPriceStep = 1.5
	MigrationPrice  = 2000
	MigrationBugs   = 3
	DevOpsBillCut   = 0.75
	MongoBugChance  = 25
	MongoBugs       = 2
	MongoPowerBoost = 0.1
	OverloadBugs    = 10

	OutageHeadroomBoost = 0.1

	ReplicaPrice      = 2500
	ReplicaBill       = 250
	ReplicaCapacity   = 0.6
	CacheDBCut        = 0.6
	CDNAppCut         = 0.7
	CDNUsersBias      = 0.5
	ContainerDiscount = 0.7

	QueueSpikeShare    = 1.0 / 3
	AutoscaleMaxExtra  = 1.0
	AutoscalePer100Req = 150
)

var tierCapacity = [4]int{0, 300, 700, 1200}
var partUpgradePrice = [4]int{0, 0, 800, 2000}

type dbInfo struct {
	Capacity int
	Bill     int
}

var databases = map[string]dbInfo{
	"sqlite":   {Capacity: 400, Bill: 0},
	"mariadb":  {Capacity: 1200, Bill: 200},
	"mongodb":  {Capacity: 1200, Bill: 300},
	"postgres": {Capacity: 1800, Bill: 300},
}

type InfraItem struct {
	ID     string `json:"id"`
	Branch string `json:"branch"`
	Fixes  string `json:"fixes"`
	Name   string `json:"name"`
	Act    int    `json:"act"`
	Price  int    `json:"price"`
	Bill   int    `json:"bill"`
	What   string `json:"what"`
	Need   string `json:"need"`
	Effect string `json:"effect"`
	Thai   string `json:"thai"`
}

var infraCatalog = []InfraItem{
	{ID: "server", Branch: "servers", Fixes: "app", Name: "App Server", Act: 1, Price: ServerPrice, Bill: 100,
		What: "A computer that runs your app and answers requests.", Need: "Your App meter is near full.",
		Effect: "Capacity = its weakest part: CPU or RAM (300 / 700 / 1,200 req/s per tier).",
		Thai:   "เครื่องที่รันแอปและตอบคำขอของผู้ใช้ รับโหลดได้เท่ากับส่วนที่อ่อนที่สุด (CPU หรือ RAM)"},
	{ID: "lb", Branch: "servers", Fixes: "app", Name: "Load Balancer", Act: 1, Price: 1500, Bill: 200,
		What: "Sits in front of your servers and hands each request to one of them.", Need: "You want a 2nd server to actually help.",
		Effect: "Without it, only your first server takes traffic.",
		Thai:   "ตัวกระจายคำขอไปยังเซิร์ฟเวอร์หลายเครื่อง ถ้าไม่มี เซิร์ฟเวอร์เครื่องที่ 2 จะไม่ได้ช่วยอะไรเลย"},
	{ID: "db:sqlite", Branch: "database", Fixes: "db", Name: "Single-file Database", Act: 1, Price: 0, Bill: 0,
		What: "The whole database is one file next to your app. Free and simple.", Need: "You're just starting.",
		Effect: "400 queries/s. No bill.",
		Thai:   "ฐานข้อมูลที่เป็นไฟล์เดียว ฟรีและง่าย เหมาะกับตอนเริ่มต้น แต่รับโหลดได้น้อย"},
	{ID: "db:postgres", Branch: "database", Fixes: "db", Name: "Strict Relational Database", Act: 1, Price: MigrationPrice, Bill: 300,
		What: "Tables with strict rules. The database refuses bad data.", Need: "You've outgrown the single file and want fewer bugs.",
		Effect: "1,800 queries/s, −1 bug per ship.",
		Thai:   "ฐานข้อมูลแบบตารางที่เข้มงวด ข้อมูลผิดรูปแบบจะถูกปฏิเสธ ช่วยลดบั๊ก รับโหลดได้มาก"},
	{ID: "db:mariadb", Branch: "database", Fixes: "db", Name: "Lightweight Relational Database", Act: 1, Price: MigrationPrice, Bill: 200,
		What: "Tables too, simpler and cheaper to run.", Need: "You want to grow without a big bill.",
		Effect: "1,200 queries/s, cheapest bill.",
		Thai:   "ฐานข้อมูลแบบตารางที่เบาและถูกกว่า เหมาะเมื่ออยากโตแต่ประหยัดค่าใช้จ่าย"},
	{ID: "db:mongodb", Branch: "database", Fixes: "db", Name: "Document Database", Act: 1, Price: MigrationPrice, Bill: 300,
		What: "Stores flexible documents instead of strict tables.", Need: "You want to build features fast.",
		Effect: "1,200 queries/s, +10% power, 25% chance of +2 bugs per ship (messy data).",
		Thai:   "ฐานข้อมูลแบบเอกสาร ยืดหยุ่น สร้างฟีเจอร์ได้เร็ว แต่ข้อมูลอาจไม่เป็นระเบียบจนเกิดบั๊ก"},
	{ID: "index", Branch: "database", Fixes: "db", Name: "Indexes", Act: 1, Price: 500, Bill: 0,
		What: "Like a book's index: the database finds rows without reading every page.", Need: "Your DB meter is filling up.",
		Effect: "−30% DB load. Cheap. Good database design first, bigger hardware second.",
		Thai:   "เหมือนสารบัญหนังสือ ฐานข้อมูลหาข้อมูลได้โดยไม่ต้องอ่านทุกแถว ลดโหลดฐานข้อมูล 30%"},
	{ID: "replica", Branch: "database", Fixes: "db", Name: "Read Replica", Act: 2, Price: ReplicaPrice, Bill: ReplicaBill,
		What: "A live copy of your database that answers read-only questions.", Need: "Your DB meter is red even with indexes.",
		Effect: "+60% DB capacity each. Not for the single-file database. Half price on the lightweight relational one.",
		Thai:   "สำเนาฐานข้อมูลที่คอยตอบคำขอแบบอ่านอย่างเดียว เพิ่มความจุฐานข้อมูล ใช้กับฐานข้อมูลไฟล์เดียวไม่ได้"},
	{ID: "cache", Branch: "speed", Fixes: "db", Name: "Cache", Act: 2, Price: 3000, Bill: 400,
		What: "Keeps popular answers in memory so the database isn't asked the same thing twice.", Need: "Your DB meter is red but App is fine.",
		Effect: "−40% DB load.",
		Thai:   "เก็บคำตอบที่ถูกถามบ่อยไว้ในหน่วยความจำ ฐานข้อมูลจึงไม่ต้องตอบคำถามเดิมซ้ำ ลดโหลดฐานข้อมูล 40%"},
	{ID: "cdn", Branch: "speed", Fixes: "app", Name: "CDN", Act: 2, Price: 2500, Bill: 300,
		What: "Copies your images and files to servers near your users.", Need: "Your App meter is red and pages feel slow.",
		Effect: "−30% App load, and the Users reviewer likes the speed.",
		Thai:   "กระจายไฟล์และรูปภาพไปไว้ที่เซิร์ฟเวอร์ใกล้ผู้ใช้ เว็บโหลดเร็วขึ้น ลดโหลดเซิร์ฟเวอร์แอป 30%"},
	{ID: "containers", Branch: "servers", Fixes: "bugs", Name: "Containers", Act: 2, Price: 1500, Bill: 100,
		What: "Packs your app with everything it needs, so it runs the same everywhere.", Need: "You hear \"it works on my machine\".",
		Effect: "−1 bug per ship, and new servers cost 30% less.",
		Thai:   "แพ็กแอปพร้อมทุกอย่างที่ต้องใช้ รันที่ไหนก็เหมือนกัน เลิกพูดว่า \"เครื่องผมรันได้นะ\""},
	{ID: "cicd", Branch: "reliability", Fixes: "bugs", Name: "CI/CD Pipeline", Act: 2, Price: 2000, Bill: 150,
		What: "Every change is tested and deployed automatically.", Need: "Bugs keep slipping into releases.",
		Effect: "−1 bug per ship.",
		Thai:   "ทุกการแก้โค้ดถูกทดสอบและนำขึ้นระบบอัตโนมัติ บั๊กหลุดไปถึงผู้ใช้น้อยลง"},
	{ID: "autoscale", Branch: "servers", Fixes: "spikes", Name: "Auto-scaling", Act: 3, Price: 4000, Bill: 200,
		What: "Rents extra servers automatically when traffic jumps, and returns them after.", Need: "Traffic spikes keep catching you out.",
		Effect: "Covers app overloads up to 2x your server capacity. You pay ฿150 per extra 100 req/s it uses.",
		Thai:   "เพิ่มเซิร์ฟเวอร์ให้อัตโนมัติเมื่อคนเข้าเยอะ แล้วคืนเมื่อเงียบลง จ่ายตามที่ใช้จริง"},
	{ID: "queue", Branch: "reliability", Fixes: "spikes", Name: "Message Queue", Act: 3, Price: 3500, Bill: 300,
		What: "Lines up sudden bursts of work so your servers handle them at their own pace.", Need: "A viral moment is about to hit.",
		Effect: "Traffic spikes from events hit at one third of their size.",
		Thai:   "ต่อคิวงานที่เข้ามาพร้อมกันจำนวนมาก ให้เซิร์ฟเวอร์ค่อย ๆ ทำทีละงาน ลดผลกระทบจากช่วงคนเข้าพุ่ง"},
	{ID: "monitoring", Branch: "reliability", Fixes: "bugs", Name: "Monitoring", Act: 1, Price: 800, Bill: 100,
		What: "Dashboards that show how busy your servers and database are.", Need: "You want to see problems before users do.",
		Effect: "Shows the App and DB meters before you ship. −1 bug per ship.",
		Thai:   "แดชบอร์ดที่บอกว่าเซิร์ฟเวอร์และฐานข้อมูลยุ่งแค่ไหน เห็นปัญหาก่อนผู้ใช้เจอ"},
	{ID: "backups", Branch: "reliability", Fixes: "bugs", Name: "Backups", Act: 1, Price: 600, Bill: 100,
		What: "A copy of your data saved somewhere else, every day.", Need: "Before someone deletes production. Not after.",
		Effect: "\"The Intern Deleted the Prod DB\" costs nothing.",
		Thai:   "สำรองข้อมูลไว้อีกที่ทุกวัน ถ้ามีคนลบฐานข้อมูลจริง ก็กู้คืนได้ทันที"},
}

var partIDs = []string{"lb", "index", "monitoring", "backups", "cache", "cdn", "containers", "cicd", "autoscale", "queue"}

type InfraPrices struct {
	Upgrade []int       `json:"upgrade"`
	Items   []InfraItem `json:"items"`
}

var infraPrices = InfraPrices{Upgrade: partUpgradePrice[2:], Items: infraCatalog}

func newInfra() *domain.StartupInfra {
	return &domain.StartupInfra{Servers: []domain.StartupServer{{CPU: 1, RAM: 1}}, DB: "sqlite"}
}

func hasPart(inf *domain.StartupInfra, id string) bool {
	return inf != nil && slices.Contains(inf.Parts, id)
}

func serverCapacity(s domain.StartupServer) int {
	return tierCapacity[min(s.CPU, s.RAM)]
}

func appCapacity(inf *domain.StartupInfra) int {
	if len(inf.Servers) == 0 {
		return 0
	}
	if !hasPart(inf, "lb") {
		return serverCapacity(inf.Servers[0])
	}
	total := 0
	for _, s := range inf.Servers {
		total += serverCapacity(s)
	}
	return total
}

func trafficLoad(run *domain.StartupRun) (app, db int) {
	spike := run.NextTraffic
	if hasPart(run.Infra, "queue") {
		spike *= QueueSpikeShare
	}
	raw := float64(run.Fans) / FansPerRequest * (1 + spike)
	appLoad, dbLoad := raw, raw
	if hasPart(run.Infra, "cdn") {
		appLoad *= CDNAppCut
	}
	if hasPart(run.Infra, "index") {
		dbLoad *= 0.7
	}
	if hasPart(run.Infra, "cache") {
		dbLoad *= CacheDBCut
	}
	return int(math.Round(appLoad)), int(math.Round(dbLoad))
}

func currentLoad(run *domain.StartupRun) *domain.StartupLoad {
	if run.Infra == nil {
		return nil
	}
	app, db := trafficLoad(run)
	return &domain.StartupLoad{App: app, AppCap: appCapacity(run.Infra), DB: db, DBCap: dbCapacity(run.Infra), NextServer: NextServerPrice(run), NextReplica: NextReplicaPrice(run)}
}

func dbCapacity(inf *domain.StartupInfra) int {
	base := databases[inf.DB].Capacity
	return base + int(math.Round(float64(base)*ReplicaCapacity*float64(inf.Replicas)))
}

func NextReplicaPrice(run *domain.StartupRun) int {
	if run.Infra == nil {
		return ReplicaPrice
	}
	price := ReplicaPrice * math.Pow(ServerPriceStep, float64(run.Infra.Replicas))
	if run.Infra.DB == "mariadb" {
		price /= 2
	}
	return int(math.Round(price/100)) * 100
}

func NextServerPrice(run *domain.StartupRun) int {
	n := 0
	if run.Infra != nil {
		n = len(run.Infra.Servers)
	}
	price := ServerPrice * math.Pow(ServerPriceStep, float64(max(0, n-1)))
	if hasPart(run.Infra, "containers") {
		price *= ContainerDiscount
	}
	return int(math.Round(price/100)) * 100
}

func catalogItem(id string) (InfraItem, bool) {
	for _, it := range infraCatalog {
		if it.ID == id {
			return it, true
		}
	}
	return InfraItem{}, false
}

func spend(run *domain.StartupRun, price int) error {
	if run.Money < price {
		return domain.ErrStartupNoFunds
	}
	run.Money -= price
	return nil
}

func InfraAction(run *domain.StartupRun, action string, index int, id string) error {
	if run.Stage != domain.StartupStageHub {
		return domain.ErrStartupWrongStage
	}
	if run.Infra == nil {
		run.Infra = newInfra()
	}
	inf := run.Infra
	switch action {
	case "server":
		if err := spend(run, NextServerPrice(run)); err != nil {
			return err
		}
		inf.Servers = append(inf.Servers, domain.StartupServer{CPU: 1, RAM: 1})
	case "cpu", "ram":
		if index < 0 || index >= len(inf.Servers) {
			return domain.ErrStartupInvalidChoice
		}
		tier := &inf.Servers[index].CPU
		if action == "ram" {
			tier = &inf.Servers[index].RAM
		}
		if *tier >= len(tierCapacity)-1 {
			return domain.ErrStartupInvalidChoice
		}
		if err := spend(run, partUpgradePrice[*tier+1]); err != nil {
			return err
		}
		*tier++
	case "part":
		it, ok := catalogItem(id)
		if !ok || !slices.Contains(partIDs, id) || hasPart(inf, id) || run.Act < it.Act {
			return domain.ErrStartupInvalidChoice
		}
		if err := spend(run, it.Price); err != nil {
			return err
		}
		inf.Parts = append(inf.Parts, id)
	case "replica":
		it, _ := catalogItem("replica")
		if run.Act < it.Act || inf.DB == "sqlite" {
			return domain.ErrStartupInvalidChoice
		}
		if err := spend(run, NextReplicaPrice(run)); err != nil {
			return err
		}
		inf.Replicas++
	case "db":
		if _, ok := databases[id]; !ok || id == inf.DB || (id == "sqlite" && inf.Replicas > 0) {
			return domain.ErrStartupInvalidChoice
		}
		if err := spend(run, MigrationPrice); err != nil {
			return err
		}
		inf.DB = id
		run.NextBugs += MigrationBugs
		addLog(run, "Migration weekend: everyone is moving data. +3 bugs next project.")
	default:
		return domain.ErrStartupInvalidChoice
	}
	return nil
}

func cloudBill(run *domain.StartupRun) int {
	inf := run.Infra
	if inf == nil {
		return 0
	}
	bill := databases[inf.DB].Bill
	for _, s := range inf.Servers {
		bill += 50 * (s.CPU + s.RAM)
	}
	for _, id := range inf.Parts {
		if it, ok := catalogItem(id); ok {
			bill += it.Bill
		}
	}
	bill += ReplicaBill * inf.Replicas
	for _, d := range run.Staff {
		if d.Role == RoleDevOps {
			return int(math.Round(float64(bill) * DevOpsBillCut))
		}
	}
	return bill
}

type infraOutcome struct {
	ratio      float64
	headroom   float64
	bugs       int
	powerMult  float64
	usersBias  float64
	scaleBill  int
	postmortem string
}

func infraEffects(run *domain.StartupRun) infraOutcome {
	out := infraOutcome{ratio: 0, powerMult: 1}
	inf := run.Infra
	if inf == nil {
		return out
	}
	load := currentLoad(run)
	if hasPart(inf, "autoscale") && load.App > load.AppCap {
		extra := min(load.App-load.AppCap, int(float64(load.AppCap)*AutoscaleMaxExtra))
		out.scaleBill = int(math.Ceil(float64(extra)/100)) * AutoscalePer100Req
		addLog(run, fmt.Sprintf("Auto-scaling rented %d extra req/s for the launch (฿%d).", extra, out.scaleBill))
		load.AppCap += extra
	}
	appRatio := float64(load.App) / float64(max(1, load.AppCap))
	dbRatio := float64(load.DB) / float64(max(1, load.DBCap))
	out.ratio = max(appRatio, dbRatio)
	out.headroom = max(0, 1-out.ratio)
	for _, id := range []string{"monitoring", "containers", "cicd"} {
		if hasPart(inf, id) {
			out.bugs--
		}
	}
	if hasPart(inf, "cdn") {
		out.usersBias = CDNUsersBias
	}
	switch inf.DB {
	case "postgres":
		out.bugs--
	case "mongodb":
		out.powerMult += MongoPowerBoost
		if rngFor(run).IntN(100) < MongoBugChance {
			out.bugs += MongoBugs
			addLog(run, "Messy documents: a field was a string in one place and a number in another. +2 bugs.")
		}
	}
	if out.ratio <= 1 {
		return out
	}
	out.bugs += int(math.Ceil((out.ratio - 1) * OverloadBugs))
	if appRatio >= dbRatio {
		out.postmortem = appPostmortem(inf, load)
	} else {
		out.postmortem = dbPostmortem(inf, load)
	}
	return out
}

func dbPostmortem(inf *domain.StartupInfra, load *domain.StartupLoad) string {
	lead := fmt.Sprintf("The database handled %d of %d queries per second.", load.DBCap, load.DB)
	switch {
	case !hasPart(inf, "index"):
		return lead + " Add indexes first: cheap, and the database stops reading every row."
	case inf.DB == "sqlite":
		return lead + " The single-file database is full. Move to a bigger database."
	case !hasPart(inf, "cache"):
		return lead + " Add a cache so popular answers never reach the database."
	default:
		return lead + " Add a read replica to share the reading work."
	}
}

func appPostmortem(inf *domain.StartupInfra, load *domain.StartupLoad) string {
	if len(inf.Servers) > 1 && !hasPart(inf, "lb") {
		return fmt.Sprintf("Your servers handled %d of %d requests per second, and only one was working: nothing was splitting the traffic. A load balancer fixes that.", load.AppCap, load.App)
	}
	weak := inf.Servers[0]
	for _, s := range inf.Servers {
		if serverCapacity(s) < serverCapacity(weak) {
			weak = s
		}
	}
	part := "CPU and RAM were"
	if weak.CPU < weak.RAM {
		part = "CPU was"
	} else if weak.RAM < weak.CPU {
		part = "RAM was"
	}
	hint := "Upgrade the weakest part, or add a server behind a load balancer."
	if weak.CPU == 3 && weak.RAM == 3 && !hasPart(inf, "cdn") {
		hint = "Your servers are maxed. A CDN would take the images and files off them."
	}
	return fmt.Sprintf("Your servers handled %d of %d requests per second. %s the bottleneck. %s", load.AppCap, load.App, part, hint)
}
