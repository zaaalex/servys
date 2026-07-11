package bitrix

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/zaaalex/servys/backend/domain"
)

// ownerTypeContact — тип объекта CRM «контакт» для crm.activity.todo.add.
const ownerTypeContact = 3

// newClientFor — сидим для тестов (в проде = NewClient, в тестах подменяется на фейковый транспорт).
var newClientFor = NewClient

// ContactFieldMap — имена полей контакта, где СТО хранит данные авто клиента.
type ContactFieldMap struct {
	Make, Model, Year, EngineCC, PowerHP, Mileage string
}

// DefaultFieldMap — конвенция по умолчанию (UF-поля контакта).
func DefaultFieldMap() ContactFieldMap {
	return ContactFieldMap{
		Make:     "UF_CRM_CAR_MAKE",
		Model:    "UF_CRM_CAR_MODEL",
		Year:     "UF_CRM_CAR_YEAR",
		EngineCC: "UF_CRM_CAR_ENGINE_CC",
		PowerHP:  "UF_CRM_CAR_POWER_HP",
		Mileage:  "UF_CRM_CAR_MILEAGE",
	}
}

// CRMFleet читает автопарк клиентов из CRM-контактов (реализует b2b.FleetSource).
type CRMFleet struct{ Fields ContactFieldMap }

func (f CRMFleet) Fleet(ctx context.Context, sc domain.ServiceCenter) ([]domain.ClientCar, error) {
	c, err := newClientFor(sc.BitrixWebhook)
	if err != nil {
		return nil, err
	}
	fm := f.Fields
	if fm.Make == "" {
		fm = DefaultFieldMap()
	}
	contacts, err := c.CrmContactList(ctx, map[string]any{
		"select": []string{"ID", "NAME", "LAST_NAME", fm.Make, fm.Model, fm.Year, fm.EngineCC, fm.PowerHP, fm.Mileage},
		"filter": map[string]any{"!" + fm.Make: ""}, // только контакты с заполненной маркой авто
	})
	if err != nil {
		return nil, err
	}
	var out []domain.ClientCar
	for _, ct := range contacts {
		mk := bstr(ct[fm.Make])
		if mk == "" {
			continue
		}
		out = append(out, domain.ClientCar{
			CRMContactID: bint64(ct["ID"]),
			ClientName:   strings.TrimSpace(bstr(ct["NAME"]) + " " + bstr(ct["LAST_NAME"])),
			Make:         mk,
			Model:        bstr(ct[fm.Model]),
			Year:         bint(ct[fm.Year]),
			EngineCC:     bint(ct[fm.EngineCC]),
			PowerHP:      bint(ct[fm.PowerHP]),
			MileageKm:    bint(ct[fm.Mileage]),
		})
	}
	return out, nil
}

// CRMRetention создаёт дело-напоминание на контакте клиента (реализует b2b.Retention).
type CRMRetention struct{ DeadlineDays int }

func (r CRMRetention) Push(ctx context.Context, sc domain.ServiceCenter, cc domain.ClientCar, a domain.Alert) (string, error) {
	c, err := newClientFor(sc.BitrixWebhook)
	if err != nil {
		return "", err
	}
	days := r.DeadlineDays
	if days <= 0 {
		days = 3
	}
	deadline := time.Now().Add(time.Duration(days) * 24 * time.Hour)
	title := fmt.Sprintf("Связаться: %s — %s %s (%s)", cc.ClientName, cc.Make, cc.Model, a.Title)
	desc := fmt.Sprintf("%s\nТекущий пробег: %d км, ориентир: %d км.", a.Description, cc.MileageKm, a.DueAtKm)
	id, err := c.CrmActivityTodoAdd(ctx, ownerTypeContact, cc.CRMContactID, deadline, title, desc, sc.ResponsibleID)
	if err != nil {
		return "", err
	}
	return strconv.FormatInt(id, 10), nil
}

// --- парсинг значений Bitrix (приходят строками либо числами) ---

func bstr(v any) string {
	switch x := v.(type) {
	case string:
		return x
	case float64:
		return strconv.FormatFloat(x, 'f', -1, 64)
	case nil:
		return ""
	default:
		return fmt.Sprint(x)
	}
}

func bint(v any) int {
	n, _ := strconv.Atoi(strings.TrimSpace(bstr(v)))
	return n
}

func bint64(v any) int64 {
	n, _ := strconv.ParseInt(strings.TrimSpace(bstr(v)), 10, 64)
	return n
}
