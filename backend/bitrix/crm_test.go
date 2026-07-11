package bitrix

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/zaaalex/servys/backend/domain"
)

// withFakeClient подменяет newClientFor на клиент, бьющий в тестовый сервер (в обход валидации IP).
func withFakeClient(t *testing.T, body string) (paths *[]string) {
	t.Helper()
	var seen []string
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		seen = append(seen, r.URL.Path)
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, body)
	}))
	t.Cleanup(srv.Close)
	orig := newClientFor
	newClientFor = func(_ string) (*Client, error) {
		return &Client{base: srv.URL + "/rest/1/tok/", http: srv.Client()}, nil
	}
	t.Cleanup(func() { newClientFor = orig })
	return &seen
}

func TestCRMFleetMapsContacts(t *testing.T) {
	body := `{"result":[
		{"ID":"10","NAME":"Иван","LAST_NAME":"Петров","UF_CRM_CAR_MAKE":"KIA","UF_CRM_CAR_MODEL":"K3","UF_CRM_CAR_YEAR":"2020","UF_CRM_CAR_MILEAGE":"95000"},
		{"ID":"11","NAME":"Без","LAST_NAME":"Авто","UF_CRM_CAR_MAKE":""}
	]}`
	withFakeClient(t, body)

	cars, err := CRMFleet{}.Fleet(context.Background(), domain.ServiceCenter{BitrixWebhook: "https://x.bitrix24.ru/rest/1/tok/"})
	if err != nil {
		t.Fatal(err)
	}
	if len(cars) != 1 {
		t.Fatalf("ожидали 1 авто (второй контакт без марки пропущен), got %d", len(cars))
	}
	c := cars[0]
	if c.CRMContactID != 10 || c.Make != "KIA" || c.Model != "K3" || c.Year != 2020 || c.MileageKm != 95000 {
		t.Fatalf("маппинг неверный: %+v", c)
	}
	if c.ClientName != "Иван Петров" {
		t.Fatalf("имя клиента: %q", c.ClientName)
	}
}

func TestCRMRetentionCreatesTodo(t *testing.T) {
	paths := withFakeClient(t, `{"result":{"id":999}}`)

	id, err := CRMRetention{}.Push(context.Background(),
		domain.ServiceCenter{BitrixWebhook: "https://x.bitrix24.ru/rest/1/tok/", ResponsibleID: 5},
		domain.ClientCar{CRMContactID: 10, ClientName: "Иван Петров", Make: "KIA", Model: "K3", MileageKm: 95000},
		domain.Alert{Title: "Моторное масло", Description: "пора", DueAtKm: 90000},
	)
	if err != nil {
		t.Fatal(err)
	}
	if id != "999" {
		t.Fatalf("ожидали remote id 999, got %q", id)
	}
	if len(*paths) == 0 || !strings.HasSuffix((*paths)[0], "/crm.activity.todo.add") {
		t.Fatalf("ожидали вызов crm.activity.todo.add, got %v", *paths)
	}
}
