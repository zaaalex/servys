.PHONY: dev-backend

# Локальный backend личного гаража. JWT_SECRET включает вход.
dev-backend:
	cd backend && JWT_SECRET=servys-local-jwt SERVYS_DEV_LOGIN=1 go run .
