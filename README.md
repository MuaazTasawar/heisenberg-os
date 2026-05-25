# Heisenberg OS

> *"Say my name."*

A full-stack criminal empire management simulation inspired by Breaking Bad and Better Call Saul. Built as a monorepo with a Go backend, FastAPI ML microservice, Next.js web frontend, and Flutter mobile app.

---

## Table of Contents

- [Overview](#overview)
- [Architecture](#architecture)
- [Tech Stack](#tech-stack)
- [Project Structure](#project-structure)
- [Getting Started](#getting-started)
- [Backend — Go](#backend--go)
- [ML Service — FastAPI](#ml-service--fastapi)
- [Web Frontend — Next.js](#web-frontend--nextjs)
- [Mobile — Flutter](#mobile--flutter)
- [API Reference](#api-reference)
- [Environment Variables](#environment-variables)
- [Git Workflow](#git-workflow)
- [Roadmap](#roadmap)

---

## Overview

Heisenberg OS is a multi-platform simulation game where players manage a fictional criminal empire. Every mechanic mirrors something from Breaking Bad or Better Call Saul:

- **Cook Lab** — Synthesize product, manage purity and yield using ML optimization
- **Money Laundering** — Route dirty money through front businesses, detect suspicious patterns
- **DEA Threat System** — An adversarial AI agent (Hank Schrader) that builds a case against you using anomaly detection and graph analysis
- **Legal Consultation** — Consult an AI-powered Saul Goodman character via the Anthropic API

This is a portfolio-grade full-stack project demonstrating real-time game state, ML-powered mechanics, JWT authentication, and cross-platform frontend development.

---

## Architecture

```
┌─────────────────────────────────────────────────────┐
│                    CLIENT LAYER                     │
│                                                     │
│   Next.js (Web)          Flutter (Mobile)           │
│   localhost:3000         Android / iOS              │
└──────────────────┬──────────────────┬───────────────┘
                   │                  │
                   ▼                  ▼
┌─────────────────────────────────────────────────────┐
│              GO BACKEND — :8080                     │
│                                                     │
│  Auth  │  Empire  │  DEA  │  Legal  │  Laundering  │
│                                                     │
│              GORM + PostgreSQL                      │
└──────────────────────────┬──────────────────────────┘
                           │
                           ▼
┌─────────────────────────────────────────────────────┐
│           FASTAPI ML SERVICE — :8000                │
│                                                     │
│  DEA Threat Model  │  Yield Optimizer  │  Anomaly  │
│  GradientBoosting  │  scipy.minimize   │  IsolationForest │
└─────────────────────────────────────────────────────┘
```

---

## Tech Stack

| Layer | Technology | Purpose |
|-------|-----------|---------|
| Backend | Go + Gin | REST API, game logic, JWT auth |
| ORM | GORM | PostgreSQL interface |
| Database | PostgreSQL 15 | Persistent game state |
| ML Service | FastAPI + Python | Threat modeling, yield optimization, anomaly detection |
| ML Libraries | scikit-learn, scipy, networkx | GradientBoosting, IsolationForest, graph analysis |
| Web | Next.js 15 + TypeScript | Dashboard, game UI |
| Styling | Tailwind + inline styles | Dark ops aesthetic |
| Mobile | Flutter | iOS and Android client |
| State (mobile) | Provider | ChangeNotifier pattern |
| AI | Anthropic API (Claude) | Saul Goodman legal consultation |
| Auth | JWT (golang-jwt) | Stateless authentication |
| Containerization | Docker | PostgreSQL local dev |

---

## Project Structure

```
heisenberg-os/
├── backend/                    # Go REST API
│   ├── cmd/
│   │   └── server/
│   │       └── main.go         # Entry point, router setup
│   ├── internal/
│   │   ├── auth/               # Register, login, JWT
│   │   ├── empire/             # Cook lab, front businesses
│   │   ├── dea/                # Threat agent, evidence tracking
│   │   ├── laundering/         # Money laundering, transactions
│   │   ├── legal/              # Saul Goodman AI via Anthropic
│   │   ├── game/               # Aggregated game state
│   │   ├── models/             # GORM models
│   │   ├── db/                 # Database init and migration
│   │   └── middleware/         # JWT auth middleware
│   ├── pkg/
│   │   └── response/           # Shared API response helpers
│   ├── .env                    # Environment variables (not committed)
│   ├── .env.example            # Environment variable template
│   ├── go.mod
│   └── go.sum
│
├── ml-service/                 # FastAPI Python ML microservice
│   ├── app/
│   │   ├── main.py             # FastAPI app, router registration
│   │   ├── models/
│   │   │   └── schemas.py      # Pydantic request/response schemas
│   │   ├── routers/
│   │   │   ├── threat.py       # DEA threat analysis endpoint
│   │   │   ├── yield_optimizer.py  # Cook yield optimization endpoint
│   │   │   └── laundering.py   # Laundering anomaly detection endpoint
│   │   └── services/
│   │       ├── dea_agent.py    # GradientBoosting threat model + evidence graph
│   │       ├── yield_model.py  # scipy optimization for cook yield
│   │       └── laundering_detector.py  # IsolationForest anomaly detection
│   ├── requirements.txt
│   └── venv/                   # Python virtual environment (not committed)
│
├── web/                        # Next.js web frontend
│   ├── app/
│   │   ├── layout.tsx          # Root layout with font imports
│   │   ├── page.tsx            # Landing page
│   │   ├── globals.css         # Global styles, CSS variables
│   │   ├── lib/
│   │   │   └── api.ts          # Axios instance with JWT interceptor
│   │   ├── login/
│   │   │   └── page.tsx        # Login screen
│   │   ├── register/
│   │   │   └── page.tsx        # Registration screen
│   │   └── dashboard/
│   │       ├── page.tsx        # Main dashboard
│   │       ├── empire/         # Cook lab page
│   │       ├── dea/            # DEA threat status page
│   │       ├── legal/          # Saul Goodman consultation page
│   │       └── launder/        # Money laundering + ledger page
│   ├── package.json
│   └── next.config.ts
│
├── mobile/                     # Flutter mobile app
│   ├── lib/
│   │   ├── main.dart           # App entry, MultiProvider setup
│   │   ├── services/
│   │   │   └── api_service.dart    # HTTP client with token injection
│   │   ├── providers/
│   │   │   ├── auth_provider.dart  # Login, logout, token management
│   │   │   └── empire_provider.dart  # Empire state, cook action
│   │   ├── screens/
│   │   │   ├── splash_screen.dart
│   │   │   ├── login_screen.dart
│   │   │   ├── dashboard_screen.dart
│   │   │   ├── empire_screen.dart
│   │   │   ├── dea_screen.dart
│   │   │   └── legal_screen.dart
│   │   └── widgets/
│   │       └── stat_card.dart  # Reusable stat display widget
│   └── pubspec.yaml
│
├── docker-compose.yml          # PostgreSQL + services
├── .gitignore
└── README.md
```

---

## Getting Started

### Prerequisites

Make sure you have all of these installed:

- [Go 1.22+](https://golang.org/dl/)
- [Python 3.11+](https://python.org)
- [Node.js 20+](https://nodejs.org)
- [Flutter 3.x+](https://flutter.dev)
- [Docker Desktop](https://docker.com/products/docker-desktop)
- [Git](https://git-scm.com)

### Clone the repo

```bash
git clone https://github.com/MuaazTasawar/heisenberg-os.git
cd heisenberg-os
```

### Start PostgreSQL

```powershell
docker run --name heisenberg-postgres `
  -e POSTGRES_USER=postgres `
  -e POSTGRES_PASSWORD=password `
  -e POSTGRES_DB=heisenberg_os `
  -p 5432:5432 `
  -d postgres:15
```

After a restart, bring it back with:

```powershell
docker start heisenberg-postgres
```

---

## Backend — Go

### Setup

```powershell
cd backend
copy .env.example .env
# Edit .env with your values
go mod tidy
```

### Run

```powershell
go run cmd/server/main.go
```

Server starts at `http://localhost:8080`

### Build for production

```powershell
go build -o heisenberg-server ./cmd/server
./heisenberg-server
```

### Database

GORM auto-migrates all tables on startup. Tables created:

- `users`
- `empires`
- `front_businesses`
- `transactions`
- `legal_cases`
- `dea_threats`

---

## ML Service — FastAPI

### Setup

```powershell
cd ml-service
python -m venv venv
.\venv\Scripts\Activate.ps1          # Windows
# source venv/bin/activate           # Mac/Linux
pip install -r requirements.txt
```

### Run

```powershell
uvicorn app.main:app --reload --port 8000
```

Service starts at `http://localhost:8000`

### Interactive Docs

Visit `http://localhost:8000/docs` for full Swagger UI with all endpoints testable in the browser.

### ML Models

| Model | Algorithm | Purpose |
|-------|-----------|---------|
| DEA Threat Agent | GradientBoostingClassifier | Predicts arrest probability from empire features |
| Yield Optimizer | scipy.optimize.minimize | Finds optimal cook effort for maximum yield |
| Laundering Detector | IsolationForest | Detects anomalous transaction patterns |

---

## Web Frontend — Next.js

### Setup

```powershell
cd web
npm install
```

### Run

```powershell
npm run dev
```

App starts at `http://localhost:3000`

### Build for production

```powershell
npm run build
npm start
```

### Environment

Create `web/.env.local`:

```
NEXT_PUBLIC_API_URL=http://localhost:8080/api/v1
```

### Pages

| Route | Description |
|-------|-------------|
| `/` | Landing page |
| `/login` | Authentication |
| `/register` | New account |
| `/dashboard` | Empire overview |
| `/dashboard/empire` | Cook lab |
| `/dashboard/dea` | DEA threat status |
| `/dashboard/legal` | Saul consultation |
| `/dashboard/launder` | Money laundering |

---

## Mobile — Flutter

### Setup

```powershell
cd mobile
flutter pub get
```

### Run

```powershell
# Android emulator
flutter run

# Specific device
flutter devices
flutter run -d <device_id>
```

### API URL Configuration

Edit `lib/services/api_service.dart`:

```dart
// Android emulator
static const String baseUrl = 'http://10.0.2.2:8080/api/v1';

// iOS simulator
static const String baseUrl = 'http://localhost:8080/api/v1';

// Physical device (use your machine's local IP)
static const String baseUrl = 'http://192.168.x.x:8080/api/v1';
```

### Build APK

```powershell
flutter build apk --release
# Output: build/app/outputs/flutter-apk/app-release.apk
```

---

## API Reference

All protected routes require the header:
```
Authorization: Bearer <jwt_token>
```

### Auth

| Method | Endpoint | Body | Description |
|--------|----------|------|-------------|
| POST | `/api/v1/auth/register` | `{username, email, password, alias}` | Create account |
| POST | `/api/v1/auth/login` | `{email, password}` | Login, returns JWT |

### Empire

| Method | Endpoint | Body | Description |
|--------|----------|------|-------------|
| GET | `/api/v1/empire` | — | Get empire state |
| POST | `/api/v1/empire/cook` | `{effort: 1-10}` | Cook a batch |
| POST | `/api/v1/empire/front` | `{name, type}` | Add front business |

### Game

| Method | Endpoint | Body | Description |
|--------|----------|------|-------------|
| GET | `/api/v1/game/state` | — | Full game state (empire + DEA + cases) |

### DEA

| Method | Endpoint | Body | Description |
|--------|----------|------|-------------|
| GET | `/api/v1/dea/threat` | — | Calculate and return current threat score |

### Laundering

| Method | Endpoint | Body | Description |
|--------|----------|------|-------------|
| POST | `/api/v1/launder` | `{amount, business_id}` | Launder dirty money |
| GET | `/api/v1/transactions` | — | Get transaction history |

### Legal

| Method | Endpoint | Body | Description |
|--------|----------|------|-------------|
| POST | `/api/v1/legal/consult` | `{charge, evidence, player_context}` | Consult Saul Goodman AI |

### ML Service

| Method | Endpoint | Body | Description |
|--------|----------|------|-------------|
| POST | `/ml/threat/analyze` | `ThreatInput` | ML threat score prediction |
| POST | `/ml/yield/optimize` | `YieldInput` | Optimal cook effort calculation |
| POST | `/ml/laundering/analyze` | `TransactionPattern` | Anomaly detection |
| GET | `/health` | — | Service health check |

---

## Environment Variables

### Backend (`backend/.env`)

```env
DATABASE_URL=postgres://postgres:password@localhost:5432/heisenberg_os
JWT_SECRET=iamtheonewhoknocks
PORT=8080
ANTHROPIC_API_KEY=your_anthropic_api_key_here
ML_SERVICE_URL=http://localhost:8000
```

### Web (`web/.env.local`)

```env
NEXT_PUBLIC_API_URL=http://localhost:8080/api/v1
```

> Get your Anthropic API key at [console.anthropic.com](https://console.anthropic.com). The Saul Goodman legal consultation feature requires it.

---

## Git Workflow

This project follows a feature-branch workflow:

```
main
├── feat/backend-setup
├── feat/ml-service
├── feat/web-frontend
└── feat/mobile
```

### Commit convention

```
feat(scope): description       # new feature
fix(scope): description        # bug fix
chore(scope): description      # tooling, deps
refactor(scope): description   # code change, no feature
```

### All commits in this project

```
chore: init repo with README and .gitignore
chore: initialize Go module for backend
chore: scaffold backend folder structure and empty files
feat(backend): add User, Empire, Transaction, LegalCase, DEAThreat models
feat(backend): add GORM database init with AutoMigrate
feat(backend): add JWT authentication middleware
feat(backend): add shared API response helper package
feat(backend): add auth — register, login, JWT token generation
feat(backend): add empire — cook batch, add front business, get empire state
feat(backend): add money laundering — launder endpoint, transaction ledger
feat(backend): add DEA threat agent — score calculation, evidence tracking
feat(backend): add Saul Goodman legal consultation via Anthropic API
feat(backend): add game state aggregator and main server entry point
chore(backend): add all Go dependencies and tidy mod files
chore(backend): add .env.example with required environment variables
fix(backend): add CORS middleware and FrontBusiness to AutoMigrate
fix(backend): resolve import paths, gorm.Model ID access, dead code in handlers
feat(ml): add Pydantic schemas for all ML endpoints
feat(ml): add DEA threat agent — GradientBoosting classifier with evidence graph
feat(ml): add yield optimizer — scipy minimize for optimal effort calculation
feat(ml): add laundering detector — IsolationForest anomaly detection
feat(ml): add FastAPI routers — threat, yield, laundering endpoints
feat(ml): add FastAPI app entry point with CORS and router registration
chore(web): initialize Next.js app with Tailwind, TypeScript, dependencies
feat(web): add Axios API service with JWT interceptor
feat(web): add landing page with Heisenberg branding
feat(web): add login and register pages
feat(web): add main dashboard with empire stats and navigation grid
feat(web): add all feature pages — empire, DEA, legal, laundering
feat(web): complete UI redesign — cinematic dark ops command center aesthetic
chore(mobile): initialize Flutter app with http, provider, shared_preferences
feat(mobile): add HTTP API service with token injection
feat(mobile): add AuthProvider and EmpireProvider with ChangeNotifier
feat(mobile): add StatCard reusable widget
feat(mobile): add all screens — splash, login, dashboard, empire, DEA, legal
feat(mobile): wire MultiProvider and app theme in main.dart
chore: add docker-compose for local dev environment
```

---

## Roadmap

### Phase 2 — Multiplayer

- [ ] WebSocket support for real-time empire updates
- [ ] Multiplayer DEA mode — one player is Hank, others run the empire
- [ ] Leaderboard — biggest empire before getting caught

### Phase 3 — Advanced ML

- [ ] Wire Go backend to call FastAPI ML endpoints directly
- [ ] Reinforcement learning DEA agent that adapts to player patterns
- [ ] NLP-based wire tap system — analyze player chat for suspicious keywords

### Phase 4 — Game Depth

- [ ] Territory map with visual control zones
- [ ] Supplier/distributor relationship graph
- [ ] Season system — each season introduces new mechanics
- [ ] Mobile push notifications for DEA alerts

---

## License

This project is for educational and portfolio purposes only.
No real criminal activity is endorsed, facilitated, or simulated in a harmful way.
All game mechanics are fictional abstractions of real-world systems.

---

*"I am not in danger, Skyler. I am the danger."*