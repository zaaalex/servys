.PHONY: dev-backend

# Полноценный локальный backend: B2B включён, секреты подходят только для разработки.
dev-backend:
	cd backend && APP_SECRET_KEY=servys-local-dev-key JWT_SECRET=servys-local-jwt ADMIN_TOKEN=dev-admin go run .
