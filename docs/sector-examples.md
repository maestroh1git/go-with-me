# Sector Examples

go-with-me is domain-agnostic. The Users domain is the starting point. Here is how you would extend it for specific industries.

### IT / SaaS

**What you add:** tenant management, feature flags, webhook delivery

**New tables:** `tenants`, `tenant_memberships`, `feature_flags`, `webhook_endpoints`, `webhook_deliveries`

**New env vars:**
```
DEFAULT_PLAN=free
WEBHOOK_SIGNING_SECRET=...
```

**New events:** `tenant.created`, `feature.enabled`, `webhook.delivered`, `webhook.failed`

**New background jobs:**
- Webhook delivery with retry (enqueue on event, worker retries up to 3 times)
- Feature flag cache warm-up on startup

**Architecture note:** add `tenant_id` to all resource tables and a middleware that resolves tenant from JWT claims or subdomain.

---

### AI / ML Platforms

**What you add:** model registry, inference job queue, usage metering

**New tables:** `ml_models`, `model_versions`, `inference_jobs`, `usage_records`

**New env vars:**
```
INFERENCE_TIMEOUT_SECONDS=30
QUOTA_MAX_REQUESTS_PER_DAY=1000
GPU_WORKER_COUNT=4
```

**New events:** `inference.queued`, `inference.completed`, `inference.failed`, `quota.exceeded`

**New background jobs:**
- Inference worker: polls `inference_jobs` where `status = 'queued'`, calls model endpoint, writes result
- Usage rollup: aggregates `usage_records` hourly for billing
- Quota reset: scheduler resets daily counters at midnight

**Architecture note:** inference jobs can be expensive — use the Worker to run them async rather than blocking HTTP handlers.

---

### Logistics / Delivery

**What you add:** driver management, shipment tracking, route optimisation jobs

**New tables:** `drivers`, `vehicles`, `shipments`, `waypoints`, `delivery_windows`

**New env vars:**
```
MAPS_API_KEY=...
ETA_REFRESH_INTERVAL_MINUTES=5
GEOFENCE_RADIUS_METERS=100
```

**New events:** `shipment.created`, `shipment.assigned`, `shipment.dispatched`, `shipment.delivered`, `driver.location_updated`

**New background jobs:**
- ETA recalculation: scheduler refreshes ETAs for in-transit shipments every 5 minutes using Maps API
- Delayed delivery alert: checks for shipments past their delivery window and notifies operations
- Driver location consumer: Redis Streams consumer reading from a device telemetry stream

---

### Agriculture

**What you add:** farm management, crop cycles, IoT sensor readings, weather-based alerts

**New tables:** `farms`, `plots`, `crop_cycles`, `sensor_devices`, `sensor_readings`, `weather_snapshots`

**New env vars:**
```
WEATHER_API_KEY=...
SENSOR_INGEST_STREAM=sensors:readings
ALERT_EMAIL=ops@farm.example.com
```

**New events:** `sensor.reading`, `alert.soil_moisture_low`, `alert.temperature_critical`, `cycle.harvested`

**New background jobs:**
- Sensor ingestion: Redis Streams consumer reading from `SENSOR_INGEST_STREAM`, writes to `sensor_readings`
- Weather snapshot: scheduler fetches current weather for each farm's coordinates every hour
- Anomaly detection: compares readings against crop-specific thresholds, emits alerts

**Architecture note:** sensor data is high-volume — use Redis Streams for ingest, write to PostgreSQL in batches rather than per-reading.

---

### Education / EdTech

**What you add:** courses, enrollments, lesson progress, certificates, scheduled reminders

**New tables:** `courses`, `lessons`, `enrollments`, `lesson_completions`, `certificates`, `reminders`

**New env vars:**
```
CERTIFICATE_SIGNING_KEY=...
REMINDER_LEAD_DAYS=3
```

**New events:** `enrollment.created`, `lesson.completed`, `course.completed`, `certificate.issued`

**New background jobs:**
- Certificate generator: listens on `course.completed`, generates PDF, stores in S3, emails learner
- Reminder scheduler: daily job finds enrollments with upcoming deadlines and enqueues reminder emails
- Progress rollup: aggregates completion percentages per enrollment for dashboard widgets

---

### Healthcare

**What you add:** patients, appointments, medical records, prescriptions, reminder notifications

**New tables:** `patients`, `appointments`, `medical_records`, `prescriptions`, `appointment_reminders`

**New env vars:**
```
RECORD_ENCRYPTION_KEY=...  # AES-256 for PII at rest
SMS_PROVIDER_API_KEY=...
APPOINTMENT_REMINDER_HOURS=24
```

**New events:** `appointment.booked`, `appointment.cancelled`, `appointment.reminder_sent`, `prescription.issued`

**New background jobs:**
- Appointment reminder: daily scheduler finds appointments in next 24 hours, sends SMS/email
- Record archival: monthly job archives records older than retention period to cold storage

**Architecture note:** encrypt `medical_records.content` at the application layer using AES-256 before storing — the encryption key must not be in the DB. Add a field-level encryption helper in `internal/lib/`.

---

### Construction / Project Management

**What you add:** projects, tasks, materials, inspections, milestone alerts

**New tables:** `projects`, `tasks`, `task_assignments`, `materials`, `inspections`, `milestones`

**New env vars:**
```
MILESTONE_ALERT_LEAD_DAYS=7
INSPECTION_CHECKLIST_BUCKET=s3://...
```

**New events:** `project.started`, `milestone.reached`, `inspection.passed`, `inspection.failed`, `task.overdue`

**New background jobs:**
- Milestone alert: daily job checks upcoming milestones, notifies project manager
- Overdue task detector: finds tasks past due date, updates status, notifies assignees
- Inspection report generator: triggers on `inspection.passed`, compiles report PDF

---

### Energy / Utilities

**What you add:** meters, periodic readings, billing cycles, consumption analytics

**New tables:** `meters`, `meter_readings`, `billing_cycles`, `bills`, `tariff_bands`

**New env vars:**
```
BILLING_CYCLE_DAY=1        # Day of month to generate bills
METER_READING_INTERVAL_HOURS=1
```

**New events:** `meter.reading`, `bill.generated`, `bill.overdue`, `usage.anomaly`

**New background jobs:**
- Meter poller: scheduler reads from meter API every hour, inserts into `meter_readings`
- Billing cycle generator: monthly job calculates consumption, applies tariffs, generates `bills`
- Anomaly detection: compares readings against historical baseline, flags outliers

---

### IoT

**What you add:** device registry, telemetry ingestion, alert rules, command dispatch

**New tables:** `devices`, `device_groups`, `telemetry` (partitioned by time), `alert_rules`, `command_queue`

**New env vars:**
```
TELEMETRY_STREAM=iot:telemetry
COMMAND_STREAM=iot:commands
TELEMETRY_RETENTION_DAYS=90
MAX_DEVICES_PER_GROUP=1000
```

**New events:** `device.registered`, `device.online`, `device.offline`, `device.alert`, `command.dispatched`, `command.acknowledged`

**New background jobs:**
- Telemetry consumer: Redis Streams consumer reading `TELEMETRY_STREAM` in batches, writing to PostgreSQL
- Alert evaluator: per-device rule evaluation on each batch of readings
- Offline detector: scheduler checks last_seen timestamps, emits `device.offline` for stale devices
- Command dispatcher: monitors `command_queue`, sends commands to devices, tracks acknowledgement

**Architecture note:** partition `telemetry` by time (`PARTITION BY RANGE (created_at)`) and create monthly partitions via a scheduler job. Drop old partitions for retention rather than running DELETE.

---

### Manufacturing / MES

**What you add:** production orders, bill of materials, work centres, quality checks

**New tables:** `production_orders`, `bom_items`, `work_centres`, `work_order_steps`, `quality_checks`

**New env vars:**
```
QUALITY_ALERT_EMAIL=quality@factory.example.com
WIP_ALERT_THRESHOLD_HOURS=4
```

**New events:** `order.created`, `order.started`, `order.completed`, `quality.passed`, `quality.rejected`, `wip.stalled`

**New background jobs:**
- WIP monitor: scheduler checks work-in-progress older than `WIP_ALERT_THRESHOLD_HOURS`, alerts floor manager
- Capacity planner: daily job calculates centre utilisation from order schedule
- Scrap tracker: aggregates rejected items per shift for quality reporting

---

### Supply Chain

**What you add:** suppliers, purchase orders, inventory, fulfilment, reorder triggers

**New tables:** `suppliers`, `purchase_orders`, `po_line_items`, `inventory`, `reorder_rules`

**New env vars:**
```
REORDER_CHECK_INTERVAL_HOURS=6
LOW_STOCK_WEBHOOK=https://...
```

**New events:** `po.created`, `po.approved`, `po.received`, `inventory.adjusted`, `inventory.low`, `reorder.triggered`

**New background jobs:**
- Reorder checker: scheduler scans `inventory` against `reorder_rules`, creates POs or sends alerts
- Receiving reconciler: compares PO line items against received goods, flags discrepancies
- Inventory snapshot: daily job snapshots inventory levels for trend reporting

---

### Business / CRM

**What you add:** contacts, deals, pipelines, activities, follow-up reminders

**New tables:** `contacts`, `organisations`, `deals`, `pipeline_stages`, `activities`, `reminders`

**New env vars:**
```
FOLLOW_UP_LEAD_DAYS=3
DEAL_INACTIVITY_DAYS=14
```

**New events:** `contact.created`, `deal.created`, `deal.stage_changed`, `deal.won`, `deal.lost`, `activity.logged`

**New background jobs:**
- Follow-up reminder: daily scheduler finds activities due for follow-up, queues reminders
- Deal inactivity alert: flags deals with no activity in `DEAL_INACTIVITY_DAYS`, notifies owner
- Pipeline report: weekly job aggregates stage counts and conversion rates
