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
	{ID: "monitoring", Branch: "reliability", Fixes: "bugs", Name: "Monitoring", Act: 1, Price: 800, Bill: 100,
		What: "Dashboards that show how busy your servers and database are.", Need: "You want to see problems before users do.",
		Effect: "Shows the App and DB meters before you ship. −1 bug per ship.",
		Thai:   "แดชบอร์ดที่บอกว่าเซิร์ฟเวอร์และฐานข้อมูลยุ่งแค่ไหน เห็นปัญหาก่อนผู้ใช้เจอ"},
	{ID: "backups", Branch: "reliability", Fixes: "bugs", Name: "Backups", Act: 1, Price: 600, Bill: 100,
		What: "A copy of your data saved somewhere else, every day.", Need: "Before someone deletes production. Not after.",
		Effect: "\"The Intern Deleted the Prod DB\" costs nothing.",
		Thai:   "สำรองข้อมูลไว้อีกที่ทุกวัน ถ้ามีคนลบฐานข้อมูลจริง ก็กู้คืนได้ทันที"},
}

var partIDs = []string{"lb", "index", "monitoring", "backups"}

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
	app = int(math.Round(float64(run.Fans) / FansPerRequest * (1 + run.NextTraffic)))
	db = app
	if hasPart(run.Infra, "index") {
		db = db * 7 / 10
	}
	return app, db
}

func currentLoad(run *domain.StartupRun) *domain.StartupLoad {
	if run.Infra == nil {
		return nil
	}
	app, db := trafficLoad(run)
	return &domain.StartupLoad{App: app, AppCap: appCapacity(run.Infra), DB: db, DBCap: databases[run.Infra.DB].Capacity}
}

func NextServerPrice(run *domain.StartupRun) int {
	n := 0
	if run.Infra != nil {
		n = len(run.Infra.Servers)
	}
	return int(math.Round(ServerPrice*math.Pow(ServerPriceStep, float64(max(0, n-1)))/100)) * 100
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
	case "db":
		if _, ok := databases[id]; !ok || id == inf.DB {
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
	postmortem string
}

func infraEffects(run *domain.StartupRun) infraOutcome {
	out := infraOutcome{ratio: 0, powerMult: 1}
	inf := run.Infra
	if inf == nil {
		return out
	}
	load := currentLoad(run)
	appRatio := float64(load.App) / float64(max(1, load.AppCap))
	dbRatio := float64(load.DB) / float64(max(1, load.DBCap))
	out.ratio = max(appRatio, dbRatio)
	out.headroom = max(0, 1-out.ratio)
	if hasPart(inf, "monitoring") {
		out.bugs--
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
	default:
		return lead + " Even a big database has a limit. The next step is a cache or read replicas, so fewer queries reach it."
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
	return fmt.Sprintf("Your servers handled %d of %d requests per second. %s the bottleneck. Upgrade the weakest part, or add a server behind a load balancer.", load.AppCap, load.App, part)
}
