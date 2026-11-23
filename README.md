# ms-payments: Pasarela de Pagos Interna Mock (EntradasYa)

Microservicio interno de procesamiento financiero y simulación determinista de pasarela de pagos para la plataforma **EntradasYa**. Procesa transacciones solicitadas exclusivamente por `ms-orders`, asegurando idempotencia transaccional y validación perimetral por secreto interno.

* **Tecnología:** Go 1.22+ / Gin Web Framework / pgxpool (PostgreSQL 16)
* **Puerto:** `8004` (Servicio interno, no expuesto en proxy público)
* **Base de datos:** `payments_db`
* **Protocolo:** REST / JSON (`application/json; charset=utf-8`)

---

## 1. Responsabilidades y Características

1. **Simulación Determinista de Pagos:** Evalúa cobros según los últimos 4 dígitos del número de tarjeta sin requerir dinero real.
2. **Garantía Estricta de Idempotencia:** Restricción e índice `UNIQUE(order_id)` en la tabla `transactions`. Si se recibe un `order_id` previamente procesado, devuelve de inmediato el resultado original (`IDEMPOTENT_HIT`) sin duplicar cobros.
3. **Seguridad Perimetral (`X-Internal-Token`):** Requiere obligatoriamente la cabecera `X-Internal-Token` coincidente con `INTERNAL_TOKEN` en todas las rutas operativas (retorna `403 Forbidden` si falta o no coincide).
4. **Observabilidad 12-Factor:** Probes estándar `/health/live` y `/health/ready` (valida conectividad con PostgreSQL).

---

## 2. Reglas Deterministas de Simulación

| Terminación de Tarjeta | Estado (`status`) | Código HTTP | Motivo (`rejection_reason`) |
| :--- | :--- | :--- | :--- |
| `...0000` | `REJECTED` | `402 Payment Required` | `FONDOS_INSUFICIENTES` |
| `...9999` | `REJECTED` | `402 Payment Required` | `TARJETA_EXPIRADA_O_BLOQUEADA` |
| `...1111` | `REJECTED` | `402 Payment Required` | `SOSPECHA_DE_FRAUDE` |
| Cualquier otra terminación | `APPROVED` | `200 OK` | Genera `auth_code` (ej: `AUTH-123456`) |

*Si `amount <= 0`, la petición es rechazada con `400 Bad Request`.*

---

## 3. Endpoints de la API

### Rutas Operativas (Requieren `X-Internal-Token`)
* `POST /api/v1/payments`: Procesa el cobro de una orden de forma idempotente y determinista.
* `GET /api/v1/payments/order/:orderId`: Consulta el detalle de la transacción registrada para una orden.

### Probes de Salud (Públicos)
* `GET /health/live`: Liveness probe (`200 OK` `{"status": "UP"}`).
* `GET /health/ready`: Readiness probe (`200 OK` `{"status": "READY"}` si la base de datos responde a `Ping()`, o `503 Service Unavailable`).

---

## 4. Variables de Entorno

Configurar en archivo `.env`:

```env
PORT=8004
DATABASE_URL=postgres://payments_user:payments_pass@localhost:5432/payments_db?sslmode=disable
INTERNAL_TOKEN=internal-service-secret-entradasya-2026
LOG_LEVEL=info
```

---

## 5. Pruebas Automatizadas y Compilación

Ejecutar la suite completa de pruebas unitarias:
```bash
go test -v ./...
```

Compilar el binario del microservicio:
```bash
go build -o ms-payments ./cmd/api
```

Ejecutar localmente:
```bash
./ms-payments
```
