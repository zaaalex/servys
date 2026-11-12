package recommender

import "github.com/zaaalex/servys/backend/domain"

type ComponentCategory = string

type ComponentSpec struct {
	Code     string
	TitleRU  string
	Category ComponentCategory
	MinKm    int
	MaxKm    int
}

type seedItem struct {
	code       string
	title      string
	category   ComponentCategory
	intervalKm int
}

var seed = []seedItem{
	{"engine_oil", "Моторное масло", domain.CategoryPrimary, 10000},
	{"oil_drain_plug_gasket", "Прокладка сливной пробки", domain.CategoryPrimary, 15000},
	{"engine_oil_filter", "Масляный фильтр", domain.CategoryPrimary, 10000},
	{"engine_air_filter", "Воздушный фильтр двигателя", domain.CategoryPrimary, 30000},
	{"cabin_filter", "Салонный фильтр", domain.CategoryPrimary, 15000},
	{"fuel_filter", "Топливный фильтр", domain.CategoryPrimary, 40000},
	{"spark_plugs", "Свечи зажигания", domain.CategoryPrimary, 40000},
	{"brake_fluid", "Тормозная жидкость", domain.CategoryPrimary, 40000},
	{"brake_pads_front", "Передние тормозные колодки", domain.CategoryPrimary, 40000},
	{"brake_pads_rear", "Задние тормозные колодки", domain.CategoryPrimary, 60000},
	{"brake_discs_front", "Передние тормозные диски", domain.CategoryPrimary, 80000},
	{"brake_discs_rear", "Задние тормозные диски", domain.CategoryPrimary, 100000},
	{"engine_coolant", "Охлаждающая жидкость", domain.CategoryPrimary, 60000},
	{"transmission_fluid", "Масло коробки передач", domain.CategoryPrimary, 60000},
	{"timing_belt", "Ремень ГРМ", domain.CategoryPrimary, 90000},
	{"accessory_belt", "Ремень навесного оборудования", domain.CategoryPrimary, 60000},
	{"battery", "Аккумулятор", domain.CategoryPrimary, 60000},
	{"tires", "Шины", domain.CategoryPrimary, 60000},
	{"wiper_blades", "Щётки стеклоочистителя", domain.CategoryPrimary, 20000},

	{"ignition_coils", "Катушки зажигания", domain.CategorySecondary, 100000},
	{"power_steering_fluid", "Жидкость ГУР", domain.CategorySecondary, 80000},
	{"differential_fluid", "Масло редуктора", domain.CategorySecondary, 60000},
	{"transfer_case_fluid", "Масло раздатки", domain.CategorySecondary, 60000},
	{"timing_chain", "Цепь ГРМ", domain.CategorySecondary, 150000},
	{"water_pump", "Помпа охлаждения", domain.CategorySecondary, 90000},
	{"thermostat", "Термостат", domain.CategorySecondary, 100000},
	{"serpentine_tensioner", "Натяжитель ремня", domain.CategorySecondary, 90000},
	{"glow_plugs", "Свечи накаливания", domain.CategorySecondary, 100000},
	{"dpf", "Сажевый фильтр", domain.CategorySecondary, 120000},
	{"pcv_valve", "Клапан PCV", domain.CategorySecondary, 80000},
	{"oxygen_sensor", "Лямбда-зонд", domain.CategorySecondary, 100000},
	{"shock_absorbers", "Амортизаторы", domain.CategorySecondary, 80000},
	{"suspension_bushings", "Сайлентблоки", domain.CategorySecondary, 90000},
	{"cv_joints", "ШРУСы и пыльники", domain.CategorySecondary, 90000},
	{"wheel_bearings", "Ступичные подшипники", domain.CategorySecondary, 100000},
	{"wheel_alignment", "Развал-схождение", domain.CategorySecondary, 30000},
	{"ac_refrigerant", "Хладагент кондиционера", domain.CategorySecondary, 60000},
	{"ac_system_check", "Кондиционер: проверка", domain.CategorySecondary, 60000},
	{"washer_fluid", "Жидкость стеклоомывателя", domain.CategorySecondary, 10000},
	{"headlight_bulbs", "Лампы фар", domain.CategorySecondary, 60000},
	{"exhaust_system", "Выхлопная система", domain.CategorySecondary, 100000},
	{"clutch", "Сцепление", domain.CategorySecondary, 120000},
	{"engine_mounts", "Подушки двигателя", domain.CategorySecondary, 0},
}

var Catalog = buildCatalog()

func buildCatalog() map[string]ComponentSpec {
	m := make(map[string]ComponentSpec, len(seed))
	for _, it := range seed {
		lo, hi := rangeFor(it.code, it.intervalKm)
		m[it.code] = ComponentSpec{
			Code:     it.code,
			TitleRU:  it.title,
			Category: it.category,
			MinKm:    lo,
			MaxKm:    hi,
		}
	}
	return m
}

func rangeFor(code string, interval int) (int, int) {
	if code == "engine_mounts" {
		return 10000, 200000
	}
	lo := interval / 4
	if lo < 1000 {
		lo = 1000
	}
	hi := interval * 3
	if hi > 300000 {
		hi = 300000
	}
	return lo, hi
}

func Lookup(code string) (ComponentSpec, bool) {
	c, ok := Catalog[code]
	return c, ok
}

func TitleFor(code string) string {
	if c, ok := Catalog[code]; ok {
		return c.TitleRU
	}
	return ""
}

func CategoryFor(code string) string {
	if c, ok := Catalog[code]; ok {
		return c.Category
	}
	return ""
}

func ValidationRanges() map[string][2]int {
	m := make(map[string][2]int, len(Catalog))
	for code, c := range Catalog {
		m[code] = [2]int{c.MinKm, c.MaxKm}
	}
	return m
}
