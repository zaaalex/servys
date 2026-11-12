package recommender

import (
	"context"
	"encoding/json"
	"log"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/zaaalex/servys/backend/domain"
	"github.com/zaaalex/servys/backend/lane"
)

// knowledgeSource читает документацию сразу и полный разбор, включая форумы, отдельно.
// Полный разбор не подменяет уже найденные строки документации: их запирает конвейер.
type knowledgeSource interface {
	ReadDocumentation(context.Context, VehicleSignature) ([]domain.Rule, error)
	Rules(context.Context, VehicleSignature) ([]domain.Rule, error)
}

type rememberEntry struct {
	At    time.Time     `json:"at"`
	Rules []domain.Rule `json:"rules"`
}

type rememberFile struct {
	Entries map[string]rememberEntry `json:"entries"`
}

// remembering отдаёт последний готовый разбор сразу.
// Пустой кэш без выбора модели читает раздел документации в этом запросе, а форумы дочитывает в фоне.
// Когда модель сама ищет, этот запрос ждёт её запросы и страницы и не запускает второй такой же проход.
type remembering struct {
	pipe    knowledgeSource
	budget  time.Duration
	ttl     time.Duration
	retry   time.Duration
	path    string
	lane    *lane.Lane
	mu      sync.Mutex
	items   map[string]rememberEntry
	flight  map[string]bool
	failed  map[string]time.Time
	emptyAt map[string]time.Time
}

func newRemembering(pipe knowledgeSource, budget, ttl time.Duration, path string) *remembering {
	if ttl <= 0 {
		ttl = 6 * time.Hour
	}
	r := &remembering{
		pipe:    pipe,
		budget:  budget,
		ttl:     ttl,
		retry:   15 * time.Minute,
		path:    path,
		items:   map[string]rememberEntry{},
		flight:  map[string]bool{},
		failed:  map[string]time.Time{},
		emptyAt: map[string]time.Time{},
	}
	r.load()
	return r
}

func (r *remembering) SetLane(l *lane.Lane) {
	if r == nil {
		return
	}
	r.mu.Lock()
	r.lane = l
	r.mu.Unlock()
}

func (r *remembering) Rules(ctx context.Context, v domain.Vehicle) ([]domain.Rule, error) {
	if r == nil {
		return nil, nil
	}
	// Свежий и недельный разбор отдаём сразу. Очередь поиска их не держит,
	// иначе экран снова пишет «идёт анализ», хотя список уже сохранён.
	if rules, ok := r.saved(v); ok {
		return rules, nil
	}
	r.mu.Lock()
	heavy := r.lane
	r.mu.Unlock()
	if heavy == nil {
		return r.rulesNow(ctx, v)
	}
	var rules []domain.Rule
	var err error
	werr := heavy.Do(ctx, lane.KindSearch, func(work context.Context) error {
		// Очередь подменяет контекст запроса на свой, без срока. Возвращаем срок,
		// чтобы поиск модели успел отступить к готовым запросам до обрыва.
		if deadline, ok := ctx.Deadline(); ok {
			var cancel context.CancelFunc
			work, cancel = context.WithDeadline(work, deadline)
			defer cancel()
		}
		rules, err = r.rulesNow(work, v)
		return nil
	})
	if werr != nil {
		return nil, werr
	}
	return rules, err
}

// saved возвращает разбор младше недели как есть. Более старый тоже возвращает
// и один раз запускает новый поиск в фоне.
func (r *remembering) saved(v domain.Vehicle) ([]domain.Rule, bool) {
	if r == nil {
		return nil, false
	}
	key := vehicleKey(v)
	if rules, ok := r.fresh(key); ok {
		return rules, true
	}
	if rules, ok := r.stale(key); ok {
		log.Printf("knowledge: saved result expired, refreshing %s", key)
		r.kick(v, key)
		return rules, true
	}
	return nil, false
}

func (r *remembering) refreshSaved(vehicles []domain.Vehicle) {
	if r == nil {
		return
	}
	for _, v := range vehicles {
		if _, ok := r.fresh(vehicleKey(v)); ok {
			continue
		}
		if _, ok := r.stale(vehicleKey(v)); ok {
			log.Printf("knowledge: weekly refresh %s", vehicleKey(v))
			r.kick(v, vehicleKey(v))
		}
	}
}

func (r *remembering) rulesNow(ctx context.Context, v domain.Vehicle) ([]domain.Rule, error) {
	if r == nil || r.pipe == nil {
		return nil, nil
	}
	key := vehicleKey(v)
	if rules, ok := r.fresh(key); ok {
		return rules, nil
	}
	if rules, ok := r.stale(key); ok {
		r.kick(v, key)
		return rules, nil
	}
	if r.heldEmpty(key) {
		return nil, nil
	}
	s := signatureFrom(v)
	if seeker, ok := r.pipe.(interface{ Seeks() bool }); ok && seeker.Seeks() {
		rules, err := r.pipe.Rules(ctx, s)
		if err != nil {
			return nil, err
		}
		if len(rules) == 0 {
			if ctx.Err() != nil {
				return nil, ctx.Err()
			}
			r.markEmpty(key)
			return nil, nil
		}
		r.store(key, rules)
		return cloneRules(rules), nil
	}
	doc, err := r.pipe.ReadDocumentation(ctx, s)
	if err != nil && ctx.Err() != nil {
		return nil, err
	}
	if err != nil {
		log.Printf("knowledge: documentation %s: %v", key, err)
	}
	if len(doc) > 0 {
		r.store(key, doc)
		r.kick(v, key)
		return cloneRules(doc), nil
	}
	rules, err := r.pipe.Rules(ctx, s)
	if err != nil {
		return nil, err
	}
	if len(rules) == 0 {
		r.markEmpty(key)
		return nil, nil
	}
	r.store(key, rules)
	return cloneRules(rules), nil
}

// Inspector сообщает, как часто источники по машине читают заново и когда был последний разбор.
type Inspector interface {
	Inspect(v domain.Vehicle) (every time.Duration, at time.Time, found bool)
}

// Inspect говорит, как часто источники по этой машине читают заново и когда был последний разбор.
// Чтение не запускает поиск.
func (rt *Runtime) Inspect(v domain.Vehicle) (every time.Duration, at time.Time, found bool) {
	every = 7 * 24 * time.Hour
	if rt == nil || rt.heavy == nil {
		return every, time.Time{}, false
	}
	if rt.heavy.ttl > 0 {
		every = rt.heavy.ttl
	}
	at, found = rt.heavy.checkedAt(vehicleKey(v))
	return every, at, found
}

func (r *remembering) checkedAt(key string) (time.Time, bool) {
	if r == nil {
		return time.Time{}, false
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	ent, ok := r.items[key]
	if !ok || ent.At.IsZero() {
		return time.Time{}, false
	}
	return ent.At, true
}

func (r *remembering) fresh(key string) ([]domain.Rule, bool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	ent, ok := r.items[key]
	if !ok || time.Since(ent.At) >= r.ttl {
		return nil, false
	}
	return cloneRules(ent.Rules), true
}

func (r *remembering) stale(key string) ([]domain.Rule, bool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	ent, ok := r.items[key]
	if !ok || len(ent.Rules) == 0 {
		return nil, false
	}
	return cloneRules(ent.Rules), true
}

func (r *remembering) heldEmpty(key string) bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	until, ok := r.emptyAt[key]
	return ok && time.Now().Before(until)
}

func (r *remembering) markEmpty(key string) {
	r.mu.Lock()
	r.emptyAt[key] = time.Now().Add(r.retry)
	r.mu.Unlock()
}

func (r *remembering) kick(v domain.Vehicle, key string) {
	r.mu.Lock()
	if r.flight[key] || time.Since(r.failed[key]) < r.retry {
		r.mu.Unlock()
		return
	}
	r.flight[key] = true
	r.mu.Unlock()
	run := func(ctx context.Context) {
		defer func() {
			r.mu.Lock()
			delete(r.flight, key)
			r.mu.Unlock()
		}()
		if r.budget > 0 {
			var cancel context.CancelFunc
			ctx, cancel = context.WithTimeout(ctx, r.budget)
			defer cancel()
		}
		rules, err := r.pipe.Rules(ctx, signatureFrom(v))
		if err != nil || len(rules) == 0 {
			log.Printf("knowledge: background %s: %v", key, err)
			r.mu.Lock()
			r.failed[key] = time.Now()
			r.mu.Unlock()
			return
		}
		r.store(key, rules)
		log.Printf("knowledge: background %s stored %d rules", key, len(rules))
	}
	r.mu.Lock()
	heavy := r.lane
	r.mu.Unlock()
	if heavy != nil {
		heavy.Later(lane.KindSearch, run)
		return
	}
	go run(context.Background())
}

func (r *remembering) store(key string, rules []domain.Rule) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.items[key] = rememberEntry{At: time.Now(), Rules: cloneRules(rules)}
	delete(r.emptyAt, key)
	delete(r.failed, key)
	r.saveLocked()
}

func (r *remembering) load() {
	if r.path == "" {
		return
	}
	raw, err := os.ReadFile(r.path)
	if err != nil {
		return
	}
	var file rememberFile
	if err := json.Unmarshal(raw, &file); err != nil {
		log.Printf("knowledge: cache %s: %v", r.path, err)
		return
	}
	for k, ent := range file.Entries {
		if len(ent.Rules) == 0 {
			continue
		}
		r.items[k] = ent
	}
}

func (r *remembering) saveLocked() {
	if r.path == "" {
		return
	}
	file := rememberFile{Entries: r.items}
	raw, err := json.Marshal(file)
	if err != nil {
		log.Printf("knowledge: cache encode: %v", err)
		return
	}
	tmp := r.path + ".tmp"
	if err := os.WriteFile(tmp, raw, 0o600); err != nil {
		log.Printf("knowledge: cache write: %v", err)
		return
	}
	if err := os.Rename(tmp, r.path); err != nil {
		log.Printf("knowledge: cache rename: %v", err)
	}
}

func vehicleKey(v domain.Vehicle) string {
	return strings.Join([]string{
		strings.ToUpper(strings.TrimSpace(v.VIN)),
		strings.ToLower(strings.TrimSpace(v.Make)),
		strings.ToLower(strings.TrimSpace(v.Model)),
		strconv.Itoa(v.Year),
		strconv.Itoa(v.EngineCC),
		strconv.Itoa(v.PowerHP),
	}, "|")
}

func signatureFrom(v domain.Vehicle) VehicleSignature {
	return VehicleSignature{
		Make:       v.Make,
		Model:      v.Model,
		Year:       v.Year,
		PowerHP:    v.PowerHP,
		VIN:        strings.ToUpper(strings.TrimSpace(v.VIN)),
		Market:     marketFromVIN(v.VIN),
		Powertrain: strings.TrimSpace(strconv.Itoa(v.EngineCC) + "cc " + strconv.Itoa(v.PowerHP) + "hp"),
	}
}

func cloneRules(in []domain.Rule) []domain.Rule {
	out := make([]domain.Rule, len(in))
	for i, rule := range in {
		out[i] = rule
		if rule.Community != nil {
			note := *rule.Community
			out[i].Community = &note
		}
	}
	return out
}
