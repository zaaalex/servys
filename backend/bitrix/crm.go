package bitrix

import (
	"context"
	"encoding/json"
	"time"
)

// CrmContactList вызывает crm.contact.list и возвращает сырые контакты (поля — строками).
func (c *Client) CrmContactList(ctx context.Context, params map[string]any) ([]map[string]any, error) {
	raw, err := c.call(ctx, "crm.contact.list", params)
	if err != nil {
		return nil, err
	}
	var list []map[string]any
	if err := json.Unmarshal(raw, &list); err != nil {
		return nil, err
	}
	return list, nil
}

// CrmActivityTodoAdd создаёт универсальное дело в таймлайне CRM (crm.activity.todo.add),
// привязанное к объекту ownerTypeID/ownerID (для контакта ownerTypeID=3). Возвращает id дела.
func (c *Client) CrmActivityTodoAdd(ctx context.Context, ownerTypeID int, ownerID int64, deadline time.Time, title, description string, responsibleID int) (int64, error) {
	params := map[string]any{
		"ownerTypeId": ownerTypeID,
		"ownerId":     ownerID,
		"deadline":    deadline.Format(time.RFC3339),
		"title":       title,
		"description": description,
	}
	if responsibleID > 0 {
		params["responsibleId"] = responsibleID
	}
	raw, err := c.call(ctx, "crm.activity.todo.add", params)
	if err != nil {
		return 0, err
	}
	var r struct {
		ID json.Number `json:"id"`
	}
	if err := json.Unmarshal(raw, &r); err != nil {
		return 0, nil // дело создано, id не распарсили — не критично
	}
	id, _ := r.ID.Int64()
	return id, nil
}
