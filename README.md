<div align="center">

# ⚗️ HEISENBERG OS

### *"Say my name."*

**A full-stack criminal-empire management simulation inspired by *Breaking Bad* and *Better Call Saul*.**
Go REST backend · FastAPI machine-learning microservice · Next.js web app · Flutter mobile app

![Go](https://img.shields.io/badge/Go-Gin%20%2B%20GORM-00ADD8?logo=go&logoColor=white)
![PostgreSQL](https://img.shields.io/badge/PostgreSQL-15-4169E1?logo=postgresql&logoColor=white)
![FastAPI](https://img.shields.io/badge/FastAPI-scikit--learn%20%2B%20scipy-009688?logo=fastapi&logoColor=white)
![Next.js](https://img.shields.io/badge/Next.js-16%20%2F%20React%2019-000000?logo=nextdotjs&logoColor=white)
![Flutter](https://img.shields.io/badge/Flutter-Provider-02569B?logo=flutter&logoColor=white)
![Claude](https://img.shields.io/badge/AI-Anthropic%20Claude-D97757)
![Auth](https://img.shields.io/badge/Auth-JWT%20HS256%20%2B%20bcrypt-C8A84B)
![Purpose](https://img.shields.io/badge/purpose-educational%20%2F%20portfolio-8A6F2E)

</div>

---

## Table of Contents

1. [Overview](#1-overview)
2. [Feature Highlights](#2-feature-highlights)
3. [System Architecture](#3-system-architecture)
4. [Request Lifecycles (Sequence Diagrams)](#4-request-lifecycles-sequence-diagrams)
5. [Game Mechanics & Formulas](#5-game-mechanics--formulas)
6. [Data Model](#6-data-model)
7. [Tech Stack](#7-tech-stack)
8. [Project Structure](#8-project-structure)
9. [Getting Started](#9-getting-started)
10. [Service Deep Dives](#10-service-deep-dives)
    - [10.1 Backend — Go](#101-backend--go)
    - [10.2 ML Service — FastAPI](#102-ml-service--fastapi)
    - [10.3 Web — Next.js](#103-web--nextjs)
    - [10.4 Mobile — Flutter](#104-mobile--flutter)
11. [API Reference](#11-api-reference)
12. [Environment Variables](#12-environment-variables)
13. [Security Notes](#13-security-notes)
14. [Implementation Status & Known Limitations](#14-implementation-status--known-limitations)
15. [Docker (Suggested Setup)](#15-docker-suggested-setup)
16. [Troubleshooting](#16-troubleshooting)
17. [Testing & Code Quality](#17-testing--code-quality)
18. [Git Workflow](#18-git-workflow)
19. [Roadmap](#19-roadmap)
20. [License & Disclaimer](#20-license--disclaimer)

---

## 1. Overview

**Heisenberg OS** is a multi-platform simulation game in which you run a fictional criminal empire. Every mechanic is a wink at the shows that inspired it, and every mechanic is backed by a real piece of engineering: a REST API, a relational data model, machine-learning models, an LLM integration, and two native-feeling clients.

| Game mechanic | Show reference | What powers it |
|---|---|---|
| **Cook Lab** | Walt & Jesse's superlab | Go service: effort → yield, dirty money, heat. FastAPI yield optimizer (scipy) |
| **Money Laundering** | Los Pollos Hermanos, the car wash | Go service: 75 % conversion, transaction ledger, flagging. FastAPI anomaly detector (IsolationForest) |
| **DEA Threat System** | Hank Schrader's investigation | Go service: weighted threat score. FastAPI classifier (GradientBoosting) + evidence graph (networkx) |
| **Legal Consultation** | Saul Goodman | Anthropic Claude API, prompted to answer in character |
| **Empire State** | The whole operation | PostgreSQL via GORM; one empire per user, lazily created |

The project is a **monorepo** with four independently runnable applications that share one HTTP/JSON contract:

```
heisenberg-os/
├── backend/      Go 1.26 · Gin · GORM · PostgreSQL       → :8080
├── ml-service/   Python 3.11+ · FastAPI · scikit-learn   → :8000
├── web/          Next.js 16 · React 19 · TypeScript      → :3000
└── mobile/       Flutter · Provider                      → Android / iOS / desktop
```

> **Scope note.** Heisenberg OS is a portfolio/learning project. It demonstrates stateful game logic, JWT auth, ML-backed mechanics, LLM integration, and cross-platform client development. See [§14](#14-implementation-status--known-limitations) for an honest account of what is fully wired and what is still scaffolding.

---

## 2. Feature Highlights

- 🔐 **Authentication** — registration with bcrypt-hashed passwords, login returning a 72-hour HS256 JWT, Gin middleware that guards every game route.
- ⚗️ **Cook Lab** — choose an effort level (1–10); the backend converts it into yield (kg), dirty money, and DEA heat.
- 💵 **Laundering** — push dirty money through a front business at a 75 % rate; large transfers (> $50,000) are **flagged** and feed the DEA threat score.
- 🚔 **DEA Threat Agent** — a lead agent (*Hank Schrader*) whose threat score is recomputed from your heat, flagged transactions, and total yield.
- ⚖️ **Call Saul** — describe a charge, the evidence, and your situation; Claude replies as Saul Goodman in under 200 words.
- 🤖 **Standalone ML microservice** — three models (threat classification, yield optimization, laundering anomaly detection) behind a Swagger-documented API.
- 🖥️ **Cinematic "dark ops" web UI** — gold-on-black command-center aesthetic with Bebas Neue / Share Tech Mono typography.
- 📱 **Flutter client** — splash → login → dashboard → Cook Lab / DEA / Saul, with Provider state management and token persistence.
- 🧱 **Clean Go layout** — `handler.go` (HTTP) / `service.go` (logic) per domain, shared response envelope, GORM auto-migration.

---

## 3. System Architecture

### 3.1 High-level diagram

```mermaid
flowchart TB
    subgraph CLIENTS["CLIENT LAYER"]
        direction LR
        WEB["Next.js 16 web app<br/>localhost:3000<br/>Axios + localStorage JWT"]
        MOB["Flutter mobile app<br/>Android / iOS<br/>http + SharedPreferences JWT"]
    end

    subgraph GO["GO BACKEND  -  Gin  -  :8080"]
        direction TB
        CORS["CORS middleware<br/>origin: localhost:3000"]
        subgraph PUB["Public routes  /api/v1"]
            AUTH["auth<br/>register / login"]
        end
        subgraph PROT["Protected routes  (JWTAuth middleware)"]
            GAME["game<br/>aggregated state"]
            EMP["empire<br/>cook / fronts"]
            LAUN["laundering<br/>launder / ledger"]
            DEA["dea<br/>threat score"]
            LEGAL["legal<br/>Saul consult"]
        end
        ORM["GORM + AutoMigrate"]
    end

    subgraph ML["FASTAPI ML SERVICE  -  :8000"]
        direction TB
        TH["/ml/threat/analyze<br/>GradientBoostingClassifier<br/>+ networkx evidence graph"]
        YO["/ml/yield/optimize<br/>scipy.optimize.minimize"]
        LD["/ml/laundering/analyze<br/>IsolationForest"]
    end

    PG[("PostgreSQL 15<br/>:5432<br/>heisenberg_os")]
    CLAUDE["Anthropic Messages API<br/>api.anthropic.com"]

    WEB -->|"HTTPS/JSON + Bearer JWT"| CORS
    MOB -->|"HTTP/JSON + Bearer JWT"| CORS
    CORS --> AUTH
    CORS --> GAME
    CORS --> EMP
    CORS --> LAUN
    CORS --> DEA
    CORS --> LEGAL
    AUTH --> ORM
    GAME --> ORM
    EMP --> ORM
    LAUN --> ORM
    DEA --> ORM
    ORM --> PG
    LEGAL -->|"x-api-key"| CLAUDE
    GO -.->|"planned integration<br/>(not yet called)"| ML

```

> The **dashed arrow** from the Go backend to the ML service is deliberate: today the ML service is a **standalone, independently callable** service. The Go backend computes its own heuristic threat score and does not yet call `/ml/*`. Wiring them together is the first item of [Roadmap Phase 3](#19-roadmap).

### 3.2 ASCII fallback (for renderers without Mermaid)

```
┌────────────────────────────────────────────────────────────────────┐
│                           CLIENT LAYER                             │
│                                                                    │
│     Next.js (Web) :3000                 Flutter (Mobile)           │
│     Axios + JWT in localStorage         http + JWT in SharedPrefs  │
└──────────────────────┬─────────────────────────┬───────────────────┘
                       │  JSON over HTTP  +  Authorization: Bearer
                       ▼                         ▼
┌────────────────────────────────────────────────────────────────────┐
│                     GO BACKEND  (Gin)  :8080                       │
│                                                                    │
│   CORS ─► /api/v1/auth/*            (public)                       │
│        └► JWTAuth ─► game · empire · dea · laundering · legal      │
│                                                                    │
│   GORM (AutoMigrate) ───────────────► PostgreSQL 15  :5432         │
│   legal ─────────────────────────────► Anthropic Messages API      │
└──────────────────────────┬─────────────────────────────────────────┘
                           ┊  (planned — not yet wired)
                           ▼
┌────────────────────────────────────────────────────────────────────┐
│                  FASTAPI ML SERVICE  :8000                         │
│                                                                    │
│  /ml/threat/analyze     GradientBoostingClassifier + networkx      │
│  /ml/yield/optimize     scipy.optimize.minimize                    │
│  /ml/laundering/analyze IsolationForest                            │
│  /health · /docs (Swagger UI)                                      │
└────────────────────────────────────────────────────────────────────┘
```

### 3.3 Ports & network map

| Component | Default address | Notes |
|---|---|---|
| Go backend | `http://localhost:8080` | Base path `/api/v1`. Override with `PORT`. |
| ML service | `http://localhost:8000` | Swagger UI at `/docs`, health at `/health`. CORS is `*`. |
| Web app | `http://localhost:3000` | The **only** origin allowed by the backend's CORS policy. |
| PostgreSQL | `localhost:5432` | DB `heisenberg_os`. |
| Android emulator → host | `http://10.0.2.2:8080/api/v1` | `10.0.2.2` is the emulator's alias for the host machine. |
| iOS simulator → host | `http://localhost:8080/api/v1` | |
| Anthropic API | `https://api.anthropic.com/v1/messages` | Called server-side only; the key never reaches a client. |

### 3.4 Backend layering

Each domain package follows the same two-file pattern, keeping HTTP concerns out of business logic:

```mermaid
flowchart LR
    subgraph cmd["cmd/server"]
        MAIN["main.go<br/>wiring, CORS, route groups"]
    end
    subgraph internal["internal/"]
        H["handler.go<br/>bind JSON, call service,<br/>write response"]
        S["service.go<br/>game rules, DB access"]
        MW["middleware/auth.go<br/>JWT parse + user_id"]
        MD["models/<br/>GORM structs"]
        DB["db/db.go<br/>connect + AutoMigrate"]
    end
    subgraph pkg["pkg/"]
        R["response/<br/>APIResponse envelope"]
    end

    MAIN --> MW
    MAIN --> H
    H --> S
    H --> R
    S --> DB
    S --> MD
    DB --> MD
```

Domain packages: `auth`, `empire`, `game`, `laundering`, `dea`, `legal`.
Cross-package dependency to know about: `laundering` and `dea` import `empire` for `GetEmpireID(userID)`.

---

## 4. Request Lifecycles (Sequence Diagrams)

### 4.1 Register & login

```mermaid
sequenceDiagram
    autonumber
    actor P as Player
    participant C as Client (web or mobile)
    participant G as Gin handler (auth)
    participant S as auth.service
    participant DB as PostgreSQL

    P->>C: Fill registration form
    C->>G: POST /api/v1/auth/register (username, email, password, alias)
    G->>G: Validate (email format, password min 6)
    G->>S: Register(...)
    S->>S: bcrypt hash (DefaultCost)
    S->>DB: INSERT user (unique username and email)
    DB-->>S: user row
    S-->>G: user (PasswordHash hidden via json:"-")
    G-->>C: 201 "Say my name"

    P->>C: Enter email and password
    C->>G: POST /api/v1/auth/login
    G->>S: Login(email, password)
    S->>DB: SELECT user WHERE email
    S->>S: bcrypt compare
    S->>S: Sign JWT HS256 (user_id, exp = now + 72h)
    S-->>G: token
    G-->>C: 200 "I am the one who knocks" + token
    C->>C: Persist token (localStorage or SharedPreferences)
```

### 4.2 Authenticated request — cooking a batch

```mermaid
sequenceDiagram
    autonumber
    actor P as Player
    participant C as Client
    participant MW as JWTAuth middleware
    participant H as empire handler
    participant S as empire.service
    participant DB as PostgreSQL

    P->>C: Set effort slider and press INITIATE COOK
    C->>MW: POST /api/v1/empire/cook with Bearer token and effort
    MW->>MW: Parse "Bearer" header, verify signature and expiry
    MW->>MW: c.Set("user_id", claims.user_id)
    MW->>H: next()
    H->>S: GetOrCreate(userID) to resolve empire ID
    S->>DB: SELECT empire WHERE user_id (create with defaults if missing)
    H->>S: Cook(empireID, effort)
    S->>DB: SELECT empire
    S->>S: yield = effort x purity/100 x 5
    S->>S: dirty += yield x 1500, heat += effort x 2.5 (cap 100)
    S->>DB: UPDATE empire
    S-->>H: updated empire
    H-->>C: 200 "Batch cooked" + empire
    C->>C: Re-render stats
```

### 4.3 Laundering and its effect on the DEA

```mermaid
sequenceDiagram
    autonumber
    actor P as Player
    participant C as Client
    participant L as laundering service
    participant D as dea service
    participant DB as PostgreSQL

    P->>C: Enter amount and business ID
    C->>L: POST /api/v1/launder
    L->>DB: SELECT empire
    alt dirty_money below amount
        L-->>C: 400 "insufficient dirty money"
    else enough funds
        L->>L: cleaned = amount x 0.75
        L->>DB: UPDATE empire (dirty down, clean up, heat += amount/100000)
        L->>DB: INSERT transaction (type laundered, flagged if amount above 50,000)
        L-->>C: 200 "Money's clean, Mr. White" + transaction
    end

    P->>C: Open DEA status page
    C->>D: GET /api/v1/dea/threat
    D->>DB: COUNT flagged transactions
    D->>D: score = heat x 0.4 + flagged x 3 + yield x 0.1 (cap 100)
    D->>DB: UPSERT dea_threat (lead agent Hank Schrader)
    D-->>C: 200 "Stay one step ahead of Hank" + threat
```

### 4.4 Consulting Saul

```mermaid
sequenceDiagram
    autonumber
    actor P as Player
    participant C as Client
    participant H as legal handler
    participant S as legal.service
    participant A as Anthropic Messages API

    P->>C: Charge, evidence, situation
    C->>H: POST /api/v1/legal/consult (JWT required)
    H->>S: ConsultSaul(charge, evidence, context)
    S->>S: Build in-character prompt (under 200 words, 3 parts)
    S->>A: POST /v1/messages (model claude-sonnet-4-20250514, max_tokens 500)
    A-->>S: content[0].text
    S-->>H: SaulResponse (advice, risk, defense)
    H-->>C: 200 "Better call Saul"
    Note over H,C: On any failure the client receives 500 "Saul is unavailable right now"
```

---

## 5. Game Mechanics & Formulas

All numbers below are taken directly from the source (`backend/internal/*/service.go`).

### 5.1 New empire defaults

An empire is created **lazily** the first time any empire-aware endpoint is called for a user.

| Field | Default |
|---|---|
| `purity_rating` | `50.0` |
| `batch_yield` | `10.0` kg |
| `heat_level` | `0.0` |
| `dirty_money` / `clean_money` | `0` / `0` |
| `territory_count` | `0` |
| `name` | `"New Empire"` if first created via `GET /empire`, otherwise `"Empire"` |

### 5.2 Cooking — `POST /empire/cook`

```
yield_gain  = effort × (purity_rating / 100) × 5
batch_yield += yield_gain
dirty_money += yield_gain × 1500
heat_level  += effort × 2.5          (clamped to ≤ 100)
```

| Effort | Yield gain @ purity 50 | Dirty money gain | Heat gain |
|---:|---:|---:|---:|
| 1 | 2.5 kg | $3,750 | +2.5 |
| 5 | 12.5 kg | $18,750 | +12.5 |
| 7 | 17.5 kg | $26,250 | +17.5 |
| 10 | 25.0 kg | $37,500 | +25.0 |

### 5.3 Laundering — `POST /launder`

```
cleaned      = amount × 0.75            (fixed 25 % loss)
dirty_money -= amount
clean_money += cleaned
heat_level  += amount / 100000
flagged      = amount > 50000
```

A `Transaction` of type `laundered` is written for `cleaned` dollars. The ledger endpoint returns the **50 most recent** transactions (newest first).

### 5.4 DEA threat score — `GET /dea/threat`

```
score = (heat_level × 0.4) + (flagged_transactions × 3.0) + (batch_yield × 0.1)
score = min(score, 100)
```

The result is **persisted** in `dea_threats` (one active row per empire, lead agent `"Hank Schrader"`) and returned to the client. The dashboard's *DEA Heat* tile reads this stored value, so it shows `0 %` until `/dea/threat` has been called at least once.

**Risk bands used by the web UI** (`/dashboard/dea`):

```mermaid
stateDiagram-v2
    direction LR
    [*] --> LOW
    LOW --> ELEVATED: score above 25
    ELEVATED --> HIGH: score above 50
    HIGH --> CRITICAL: score above 75
    CRITICAL --> HIGH: score at or below 75
    HIGH --> ELEVATED: score at or below 50
    ELEVATED --> LOW: score at or below 25

    LOW: LOW - green - no active investigation
    ELEVATED: ELEVATED - gold - Hank is sniffing around
    HIGH: HIGH - orange - a case is being built
    CRITICAL: CRITICAL - red - contact Saul
```

### 5.5 Worked example

A player cooks four batches at effort 10, then launders $100,000:

| Step | Purity | Yield (kg) | Dirty $ | Clean $ | Heat | Flagged tx |
|---|---:|---:|---:|---:|---:|---:|
| New empire | 50 | 10 | 0 | 0 | 0 | 0 |
| Cook ×4 @ effort 10 | 50 | 110 | 150,000 | 0 | 100 (capped) | 0 |
| Launder $100,000 | 50 | 110 | 50,000 | 75,000 | 101 *(laundering does not clamp)* | 1 |

`score = 101 × 0.4 + 1 × 3 + 110 × 0.1 = 40.4 + 3 + 11 = 54.4` → **HIGH**.

### 5.6 Saul Goodman (LLM)

The prompt asks the model to respond *in character* with (1) legal advice, (2) a risk level, and (3) a defense strategy in under 200 words. The whole reply is returned as `advice`. The `risk` (`"medium"`) and `defense` (`"See Saul's advice above"`) fields are currently static placeholders.

---

## 6. Data Model

GORM auto-migrates six tables on every backend start. All models embed `gorm.Model` (`ID`, `CreatedAt`, `UpdatedAt`, `DeletedAt` → soft deletes).

```mermaid
erDiagram
    USERS ||--|| EMPIRES : "owns (user_id)"
    EMPIRES ||--o{ FRONT_BUSINESSES : "operates (empire_id)"
    EMPIRES ||--o{ TRANSACTIONS : "records (empire_id)"
    EMPIRES ||--o{ LEGAL_CASES : "faces (empire_id)"
    EMPIRES ||--o{ DEA_THREATS : "tracked by (empire_id)"
    FRONT_BUSINESSES ||--o{ TRANSACTIONS : "business_id (soft reference)"

    USERS {
        uint id PK
        string username UK "not null"
        string email UK "not null"
        string password_hash "bcrypt, never serialized"
        string alias "street name"
    }
    EMPIRES {
        uint id PK
        uint user_id FK
        string name
        float purity_rating
        float batch_yield
        float dirty_money
        float clean_money
        float heat_level
        int territory_count
    }
    FRONT_BUSINESSES {
        uint id PK
        uint empire_id FK
        string name
        string type
        float revenue
        float suspicion
        bool is_active
    }
    TRANSACTIONS {
        uint id PK
        uint empire_id FK
        float amount
        string type "dirty, clean, laundered"
        string description
        uint business_id
        bool flagged
    }
    LEGAL_CASES {
        uint id PK
        uint empire_id FK
        string charge
        string evidence
        string status "open, dismissed, convicted"
        int severity
    }
    DEA_THREATS {
        uint id PK
        uint empire_id FK
        float threat_score
        int evidence_count
        string lead_agent
        bool is_active
    }
```

| Table | Written by | Read by |
|---|---|---|
| `users` | `POST /auth/register` | `POST /auth/login` |
| `empires` | `GET /empire` (lazy create), cook, launder | everything |
| `front_businesses` | `POST /empire/front` | `GET /empire`, `GET /game/state` (preloaded) |
| `transactions` | `POST /launder` | `GET /transactions`, DEA flagged count |
| `dea_threats` | `GET /dea/threat` | `GET /dea/threat`, `GET /game/state` |
| `legal_cases` | *(model + table exist, no endpoint writes it yet)* | `GET /game/state` |

> **JSON field casing.** Because `gorm.Model` carries no JSON tags, its fields serialize as `ID`, `CreatedAt`, `UpdatedAt`, `DeletedAt` (capitalized), while the rest of each model uses snake_case (`dirty_money`, `heat_level`, …).

---

## 7. Tech Stack

| Layer | Technology | Version (from manifests) | Purpose |
|---|---|---|---|
| Backend language | Go | `go 1.26.1` (go.mod) | REST API & game logic |
| HTTP framework | Gin | v1.12.0 | Routing, binding, middleware |
| CORS | gin-contrib/cors | v1.7.7 | Browser cross-origin policy |
| ORM | GORM + pgx driver | gorm v1.31.1, driver/postgres v1.6.0 | PostgreSQL access, auto-migration |
| Auth | golang-jwt/jwt v5, x/crypto bcrypt | v5.3.1 | JWT (HS256) + password hashing |
| Config | godotenv | v1.5.1 | `.env` loading |
| Database | PostgreSQL | 15 | Persistent game state |
| ML service | FastAPI + Uvicorn | 0.136.3 / 0.48.0 | ML microservice |
| ML libraries | scikit-learn, scipy, networkx, numpy | 1.8.0 / 1.17.1 / 3.6.1 / 2.4.6 | Classification, optimization, anomaly detection, graphs |
| Validation | Pydantic | 2.13.4 | Request/response schemas |
| Web framework | Next.js (App Router) + React | 16.2.6 / 19.2.4 | Web client |
| Web language | TypeScript | ^5 | Type safety |
| Web HTTP | Axios | ^1.16.1 | API client with JWT interceptor |
| Web styling | Inline styles + CSS variables (Tailwind 4 installed) | — | "Dark ops" aesthetic |
| Mobile | Flutter / Dart | Dart SDK `^3.10.7` | iOS & Android client |
| Mobile state | provider | ^6.1.5+1 | `ChangeNotifier` pattern |
| Mobile storage / HTTP | shared_preferences, http | ^2.5.5 / ^1.6.0 | Token persistence, API calls |
| LLM | Anthropic Messages API | `claude-sonnet-4-20250514` (hard-coded) | Saul Goodman |

> Several web dependencies — `@tanstack/react-query`, `zustand`, `recharts`, `lucide-react` — are declared in `package.json` but **not yet used** by any page. They are reserved for upcoming data-fetching, state, and charting work.

---

## 8. Project Structure

```
heisenberg-os/
├── backend/                         # Go REST API
│   ├── cmd/server/main.go           # Entry point: env, DB init, CORS, route groups
│   ├── internal/
│   │   ├── auth/                    # handler.go + service.go — register, login, JWT issue
│   │   ├── empire/                  # handler.go + service.go — empire state, cook, fronts
│   │   ├── game/                    # handler.go + service.go — aggregated game state
│   │   ├── laundering/              # handler.go + service.go — launder + ledger
│   │   ├── dea/                     # handler.go + agent.go   — threat score calculation
│   │   ├── legal/                   # handler.go + service.go — Saul via Anthropic API
│   │   ├── middleware/auth.go       # JWTAuth(): validates Bearer token, sets user_id
│   │   ├── models/                  # user.go, empire.go, transaction.go, case.go
│   │   └── db/db.go                 # gorm.Open + AutoMigrate
│   ├── pkg/response/response.go     # OK()/Fail() + APIResponse envelope
│   ├── go.mod / go.sum
│   └── .env                         # local secrets (git-ignored)
│
├── ml-service/                      # FastAPI microservice
│   ├── app/
│   │   ├── main.py                  # App, CORS, router registration, /health
│   │   ├── models/schemas.py        # Pydantic request/response models
│   │   ├── routers/
│   │   │   ├── threat.py            # POST /ml/threat/analyze
│   │   │   ├── yield_optimizer.py   # POST /ml/yield/optimize
│   │   │   └── laundering.py        # POST /ml/laundering/analyze
│   │   └── services/
│   │       ├── dea_agent.py         # GradientBoosting threat model + evidence graph
│   │       ├── yield_model.py       # scipy yield optimizer
│   │       └── laundering_detector.py  # IsolationForest + rule-based patterns
│   └── requirements.txt
│
├── web/                             # Next.js 16 App Router
│   ├── app/
│   │   ├── layout.tsx               # Root layout + Google Fonts
│   │   ├── globals.css              # Design tokens (CSS variables)
│   │   ├── page.tsx                 # Landing page
│   │   ├── lib/api.ts               # Axios instance + JWT request interceptor
│   │   ├── login/page.tsx
│   │   ├── register/page.tsx
│   │   └── dashboard/
│   │       ├── page.tsx             # Empire overview + navigation grid
│   │       ├── empire/page.tsx      # Cook Lab
│   │       ├── launder/page.tsx     # Laundering + ledger
│   │       ├── dea/page.tsx         # Threat gauge
│   │       └── legal/page.tsx       # Saul consultation
│   ├── AGENTS.md / CLAUDE.md        # Notes for AI coding agents (see §10.3)
│   └── package.json, tsconfig.json, next.config.ts, eslint.config.mjs, postcss.config.mjs
│
├── mobile/                          # Flutter app (id: com.heisenbergos.mobile)
│   ├── lib/
│   │   ├── main.dart                # MultiProvider + dark gold theme
│   │   ├── services/api_service.dart
│   │   ├── providers/               # auth_provider.dart, empire_provider.dart
│   │   ├── screens/                 # splash, login, dashboard, empire, dea, legal
│   │   └── widgets/stat_card.dart
│   ├── test/widget_test.dart
│   ├── android/ ios/ linux/ macos/ windows/ web/   # Flutter platform scaffolds
│   └── pubspec.yaml
│
├── .gitignore                       # node_modules, .env, __pycache__, venv, build, .dart_tool …
└── README.md
```

---

## 9. Getting Started

### 9.1 Prerequisites

| Tool | Version | Needed for |
|---|---|---|
| [Go](https://go.dev/dl/) | matches `go.mod` (`1.26.1`) | Backend |
| [Python](https://python.org) | 3.11+ (required by the pinned numpy / scikit-learn) | ML service |
| [Node.js](https://nodejs.org) | 20+ | Web (Next.js 16) |
| [Flutter](https://flutter.dev) | stable with Dart `^3.10.7` | Mobile |
| [Docker](https://docker.com/products/docker-desktop) | any recent | PostgreSQL |
| [Git](https://git-scm.com) | any | Cloning |
| Anthropic API key | — | *Optional* — only for the Saul Goodman feature |

### 9.2 Quick start (4 terminals)

```bash
# 0 — clone
git clone https://github.com/MuaazTasawar/heisenberg-os.git
cd heisenberg-os
```

| Terminal | Command | Result |
|---|---|---|
| **1 · Database** | see [9.3](#93-start-postgresql) | Postgres on `:5432` |
| **2 · Backend** | `cd backend && go run cmd/server/main.go` | API on `:8080` |
| **3 · ML service** *(optional)* | `cd ml-service && uvicorn app.main:app --reload --port 8000` | ML on `:8000` |
| **4 · Web** | `cd web && npm install && npm run dev` | UI on `:3000` |

Then open **http://localhost:3000**, click **NEW IDENTITY**, register, log in, and start cooking.

### 9.3 Start PostgreSQL

**PowerShell**

```powershell
docker run --name heisenberg-postgres `
  -e POSTGRES_USER=postgres `
  -e POSTGRES_PASSWORD=password `
  -e POSTGRES_DB=heisenberg_os `
  -p 5432:5432 `
  -d postgres:15
```

**macOS / Linux**

```bash
docker run --name heisenberg-postgres \
  -e POSTGRES_USER=postgres \
  -e POSTGRES_PASSWORD=password \
  -e POSTGRES_DB=heisenberg_os \
  -p 5432:5432 \
  -d postgres:15
```

After a reboot, bring it back with `docker start heisenberg-postgres`.

### 9.4 Backend (Go)

```bash
cd backend
cp .env.example .env          # Windows: copy .env.example .env
# if .env.example is missing, create .env by hand — see §12
go mod tidy
go run cmd/server/main.go     # → http://localhost:8080
```

On start the server loads `.env` (logging "No .env file found" if absent), connects to Postgres, **auto-migrates all tables**, and serves `/api/v1`.

Production-style build:

```bash
go build -o heisenberg-server ./cmd/server
./heisenberg-server
```

> ⚠️ **Set `GIN_MODE=release`** for anything beyond local development.

### 9.5 ML service (Python)

```powershell
cd ml-service
python -m venv venv
.\venv\Scripts\Activate.ps1          # Windows PowerShell
# source venv/bin/activate           # macOS / Linux
pip install -r requirements.txt
uvicorn app.main:app --reload --port 8000
```

- Swagger UI: **http://localhost:8000/docs**
- Health check: `GET http://localhost:8000/health` → `{"status":"alive","message":"I am the danger"}`

> `requirements.txt` is a full `pip freeze` and is saved as **UTF-16 LE with CRLF line endings** (typical of PowerShell redirection). Modern pip reads it fine; if an older tool chokes, re-save it as UTF-8. It also contains packages the code does not import (`SQLAlchemy`, `passlib`, `python-jose`, …).

### 9.6 Web (Next.js)

```bash
cd web
npm install
echo "NEXT_PUBLIC_API_URL=http://localhost:8080/api/v1" > .env.local
npm run dev                  # → http://localhost:3000
```

Production build: `npm run build && npm start`. Lint: `npm run lint`.

### 9.7 Mobile (Flutter)

```bash
cd mobile
flutter pub get
flutter devices              # list targets
flutter run                  # or: flutter run -d <device_id>
```

Point the app at your backend by editing `lib/services/api_service.dart`:

```dart
// Android emulator (default in the repo)
static const String baseUrl = 'http://10.0.2.2:8080/api/v1';
// iOS simulator
static const String baseUrl = 'http://localhost:8080/api/v1';
// Physical device — your computer's LAN IP, same Wi-Fi network
static const String baseUrl = 'http://192.168.x.x:8080/api/v1';
```

Release APK: `flutter build apk --release` → `build/app/outputs/flutter-apk/app-release.apk`.
**Read [§16](#16-troubleshooting) first** — the committed Android manifest does not yet declare the `INTERNET` permission for release builds.

### 9.8 End-to-end smoke test (curl)

```bash
API=http://localhost:8080/api/v1

# 1. Register
curl -s -X POST $API/auth/register -H 'Content-Type: application/json' \
  -d '{"username":"walter","email":"walt@example.com","password":"iamthedanger","alias":"Heisenberg"}'

# 2. Login → copy the token
TOKEN=$(curl -s -X POST $API/auth/login -H 'Content-Type: application/json' \
  -d '{"email":"walt@example.com","password":"iamthedanger"}' | python3 -c 'import sys,json;print(json.load(sys.stdin)["data"]["token"])')

# 3. Cook four max-effort batches
for i in 1 2 3 4; do
  curl -s -X POST $API/empire/cook -H "Authorization: Bearer $TOKEN" \
    -H 'Content-Type: application/json' -d '{"effort":10}' >/dev/null
done

# 4. Add a front business, then launder $100,000 through it
curl -s -X POST $API/empire/front -H "Authorization: Bearer $TOKEN" \
  -H 'Content-Type: application/json' -d '{"name":"Los Pollos Hermanos","type":"restaurant"}'
curl -s -X POST $API/launder -H "Authorization: Bearer $TOKEN" \
  -H 'Content-Type: application/json' -d '{"amount":100000,"business_id":1}'

# 5. Check how worried Hank should be, then view everything
curl -s $API/dea/threat -H "Authorization: Bearer $TOKEN"
curl -s $API/game/state -H "Authorization: Bearer $TOKEN"
```

<details>
<summary><b>PowerShell equivalent (login + cook)</b></summary>

```powershell
$api = "http://localhost:8080/api/v1"
$login = Invoke-RestMethod -Method Post -Uri "$api/auth/login" -ContentType "application/json" `
  -Body '{"email":"walt@example.com","password":"iamthedanger"}'
$h = @{ Authorization = "Bearer $($login.data.token)" }
Invoke-RestMethod -Method Post -Uri "$api/empire/cook" -Headers $h -ContentType "application/json" -Body '{"effort":10}'
```
</details>

---

## 10. Service Deep Dives

### 10.1 Backend — Go

#### Startup sequence (`cmd/server/main.go`)

1. `godotenv.Load()` — read `.env` if present.
2. `db.Init()` — open Postgres from `DATABASE_URL` (fatal on failure), `AutoMigrate` the six models.
3. `gin.Default()` — logger + panic recovery; `SetTrustedProxies(["127.0.0.1"])`.
4. CORS: origin `http://localhost:3000`, methods `GET POST PUT DELETE OPTIONS`, headers `Authorization, Content-Type`, credentials allowed.
5. Mount public routes (`auth`) on `/api/v1`, then a **protected group** wrapped in `middleware.JWTAuth()` for `game`, `empire`, `dea`, `laundering`, `legal`.
6. Listen on `PORT` (default `8080`).

#### Authentication flow

- **Register**: `username` (required, unique), `email` (required, valid format, unique), `password` (required, min 6), `alias` (optional). Password is bcrypt-hashed with `bcrypt.DefaultCost`.
- **Login**: looks up by email, compares the hash, returns an HS256 JWT with claims `{ user_id, exp }` valid for **72 hours**.
- **Middleware** (`JWTAuth`): requires `Authorization: Bearer <token>`, validates signature/expiry with `JWT_SECRET`, and stores `user_id` (`uint`) in the Gin context for handlers (`c.GetUint("user_id")`).

#### Response envelope (`pkg/response`)

Every handler responds through two helpers:

```jsonc
// success — response.OK(c, status, message, data)
{ "success": true,  "message": "Batch cooked", "data": { /* payload */ } }

// failure — response.Fail(c, status, err)
{ "success": false, "message": "", "error": "insufficient dirty money" }
```

> The JWT middleware is the one exception: it aborts with a bare `{"error": "missing token"}` or `{"error": "invalid token"}` (HTTP 401) rather than the envelope.

#### Personality built into the API

| Endpoint | Success message |
|---|---|
| `POST /auth/register` | `Say my name` |
| `POST /auth/login` | `I am the one who knocks` |
| `GET /empire` | `Empire retrieved` |
| `POST /empire/cook` | `Batch cooked` |
| `POST /empire/front` | `Front business added — keep it clean` |
| `GET /game/state` | `Game state loaded` |
| `POST /launder` | `Money's clean, Mr. White` |
| `GET /transactions` | `Transaction ledger` |
| `GET /dea/threat` | `Stay one step ahead of Hank` |
| `POST /legal/consult` | `Better call Saul` |

### 10.2 ML Service — FastAPI

A self-contained service (`app/main.py`) exposing three routers under `/ml/*`, a `/health` probe, and auto-generated OpenAPI docs at `/docs` and `/redoc`.

#### Models at a glance

| Service | Algorithm | Inputs | How it works |
|---|---|---|---|
| **DEA Threat Agent** (`dea_agent.py`) | `GradientBoostingClassifier(n_estimators=100)` + `networkx.DiGraph` | heat, flagged tx, batch yield, territories, days active | Trained at start-up on 500 seeded synthetic samples; predicted probability of the "detected" class ×100 is the threat score |
| **Yield Optimizer** (`yield_model.py`) | `scipy.optimize.minimize` | purity, effort, supply quality, lab capacity | Predicts yield and searches effort ∈ [0, 100] that maximizes it |
| **Laundering Detector** (`laundering_detector.py`) | `IsolationForest(contamination=0.15)` + rules | transaction amounts, revenues, time gaps, business type | Anomaly flag from mean/std/time-gap features plus four human-readable pattern rules |

#### Endpoint contracts & real example outputs

The examples below were produced by running the service's own code.

<details open>
<summary><b>POST /ml/threat/analyze</b></summary>

Feature scaling used internally: `heat/100`, `flagged/20`, `yield/1000`, `territories/10`, `days/365`, plus a small random noise feature (`rand × 0.1`).

```jsonc
// Request — ThreatInput
{ "empire_id": 1, "heat_level": 100, "flagged_transactions": 20,
  "batch_yield": 1000, "territory_count": 10, "days_active": 365 }

// Response — ThreatOutput
{ "threat_score": 99.94, "risk_level": "critical",
  "prediction": "They know. It's over — unless you talk to Saul.",
  "recommendation": "Full operational freeze. Destroy evidence. Run." }
```

| Score | `risk_level` | Prediction | Recommendation |
|---|---|---|---|
| < 25 | `low` | Hank suspects nothing. Stay clean. | Continue operations. Keep front businesses active. |
| 25 – 50 | `medium` | Hank is sniffing around. Reduce activity. | Pause large transactions. Increase laundering ratio. |
| 50 – 75 | `high` | A case is being built. Lawyer up. | Shut down one territory. Consult Saul immediately. |
| ≥ 75 | `critical` | They know. It's over — unless you talk to Saul. | Full operational freeze. Destroy evidence. Run. |

> Because the model is trained on synthetic uniform data with a linear-threshold label, it behaves almost like a step function: moderate inputs (e.g. heat 62.5, 4 flagged tx, 120 kg) score ≈ 0.5 (*low*), while saturated inputs score ≈ 99+ (*critical*).
</details>

<details>
<summary><b>POST /ml/yield/optimize</b></summary>

```
predicted_yield = capacity × (supply/100) × (effort/100) × (1 + (purity − 50)/200)
efficiency      = predicted_yield / capacity × 100
purity_proj     = min(100, purity + optimal_effort × 0.05)
```

```jsonc
// Request — YieldInput
{ "current_purity": 50, "effort_level": 60, "supply_quality": 80, "lab_capacity": 100 }

// Response — YieldOutput
{ "predicted_yield": 48.0, "optimal_effort": 100.0,
  "purity_projection": 55.0, "efficiency_score": 48.0 }
```
</details>

<details>
<summary><b>POST /ml/laundering/analyze</b></summary>

Feature vector: `[mean(amounts), std(amounts), mean(time_gaps)]`. `business_revenues` is accepted but currently unused.

Pattern rules:

| Rule | Trigger |
|---|---|
| Large single transaction detected | any amount > 50,000 |
| Structuring pattern — identical amounts | > 1 amount and `std < 100` |
| High-frequency transactions | mean time gap < 1 |
| Revenue inconsistent with business type | `car_wash` / `nail_salon` with mean amount > 20,000 |

`risk_score = min(100, 20 × patterns + (40 if anomaly else 0))`; recommendation is *"Reduce transaction sizes"* above 50, otherwise *"Patterns look normal"*. An empty `amounts` list returns `risk_score 0` and `"No data"`.

```jsonc
// Request — TransactionPattern (classic structuring)
{ "amounts": [9500, 9500, 9500, 9500],
  "business_revenues": [12000, 11800, 12100, 12050],
  "time_gaps": [0.5, 0.4, 0.6, 0.5],
  "business_type": "car_wash" }

// Response — LaunderingRisk
{ "risk_score": 80.0, "anomaly_detected": true,
  "suspicious_patterns": ["Structuring pattern — identical amounts",
                          "High-frequency transactions"],
  "recommendation": "Reduce transaction sizes" }
```
</details>

#### Calling it directly

```bash
curl -s -X POST http://localhost:8000/ml/threat/analyze \
  -H 'Content-Type: application/json' \
  -d '{"empire_id":1,"heat_level":80,"flagged_transactions":10,"batch_yield":600,"territory_count":6,"days_active":200}'
```

### 10.3 Web — Next.js

#### Route map

```mermaid
flowchart LR
    LAND["/<br/>Landing"] --> LOGIN["/login<br/>I AM THE ONE WHO KNOCKS"]
    LAND --> REG["/register<br/>SAY MY NAME"]
    REG -->|"201 then redirect"| LOGIN
    LOGIN -->|"store JWT in localStorage"| DASH["/dashboard<br/>Empire overview"]
    DASH --> COOK["/dashboard/empire<br/>Cook Lab"]
    DASH --> LAUN["/dashboard/launder<br/>Los Pollos laundering"]
    DASH --> DEA["/dashboard/dea<br/>Operation Heisenberg"]
    DASH --> SAUL["/dashboard/legal<br/>Call Saul"]
    DASH -->|"LOGOUT clears token"| LOGIN
```

| Route | Backend calls | What it shows |
|---|---|---|
| `/` | — | Cinematic landing page, **ENTER** / **NEW IDENTITY** buttons. The "99.1 % purity / $47M laundered / 0 convictions" stats are decorative. |
| `/login` | `POST /auth/login` | Email + password; Enter key submits; stores `token` in `localStorage`. |
| `/register` | `POST /auth/register` | Username, alias, email, password; redirects to `/login`. |
| `/dashboard` | `GET /game/state` | Purity, dirty money, clean money, DEA heat tiles; yield and territory cards; navigation grid. Redirects to `/login` if no token or on failure. |
| `/dashboard/empire` | `GET /empire`, `POST /empire/cook` | Effort slider (1–10) with live projections for yield, heat, and earnings. |
| `/dashboard/launder` | `GET /transactions`, `POST /launder` | Amount + business ID form, 75 % preview, scrolling ledger with red `FLAGGED` badges. |
| `/dashboard/dea` | `GET /dea/threat` | 96-px threat score, risk badge, lead agent, evidence count, animated gauge, advice banner. |
| `/dashboard/legal` | `POST /legal/consult` | Charge / evidence / situation form → Saul's reply panel. |

#### API client (`app/lib/api.ts`)

An Axios instance with `baseURL = NEXT_PUBLIC_API_URL || "http://localhost:8080/api/v1"` and a request interceptor that attaches `Authorization: Bearer <localStorage.token>` to every call.

#### Design system ("dark ops command center")

CSS variables in `app/globals.css`:

| Token | Value | Use |
|---|---|---|
| `--void` / `--black` / `--panel` | `#060606` / `#0a0a0a` / `#111111` | Backgrounds |
| `--border` / `--border-hot` | `#1e1e1e` / `#2a2a2a` | Dividers |
| `--gold` / `--gold-dim` / `--gold-glow` | `#c8a84b` / `#8a6f2e` / `#e8c56a` | Primary accent |
| `--red` / `--red-dim` | `#c0392b` / `#7a1f15` | Danger, DEA, dirty money |
| `--blue` | `#2980b9` | Purity ("blue sky") |
| `--green` / `--green-bright` | `#1a7a3a` / `#27ae60` | Clean money, success |
| `--text` / `--text-dim` / `--text-muted` | `#e8e4d8` / `#7a7570` / `#3a3530` | Typography |

Fonts (Google Fonts): **Bebas Neue** (headings), **Share Tech Mono** (labels, data), **Barlow** (body).

#### Notes for contributors

- `web/AGENTS.md` (imported by `CLAUDE.md`) warns that this Next.js version has breaking changes and instructs readers to consult `node_modules/next/dist/docs/` before writing Next-specific code.
- Pages are client components that style themselves inline; there is not yet a shared component library.
- Only `/dashboard` guards for a missing token; the sub-pages rely on the API returning errors.

### 10.4 Mobile — Flutter

#### Navigation & state

```mermaid
flowchart TB
    subgraph Providers["MultiProvider (main.dart)"]
        AP["AuthProvider<br/>isLoggedIn, login(), logout()"]
        EP["EmpireProvider<br/>empire, threat, loadState(), cook()"]
    end
    API["ApiService<br/>static get/post + token headers<br/>baseUrl http://10.0.2.2:8080/api/v1"]
    SP[("SharedPreferences<br/>key: token")]

    SPLASH["SplashScreen<br/>2 s delay, check token"] -->|"token present"| DASH["DashboardScreen"]
    SPLASH -->|"no token"| LOGIN["LoginScreen<br/>I AM THE DANGER"]
    LOGIN -->|"login ok"| DASH
    DASH --> COOK["EmpireScreen<br/>The Cook Lab"]
    DASH --> DEA["DEAScreen<br/>Operation Heisenberg"]
    DASH --> SAUL["LegalScreen<br/>Better Call Saul"]

    AP --> API
    EP --> API
    API --> SP
    LOGIN -.-> AP
    DASH -.-> EP
    COOK -.-> EP
    DEA -.-> EP
    SAUL --> API
```

| Screen | Behavior |
|---|---|
| **Splash** | Shows the HEISENBERG / OS wordmark for 2 s, reads `token` from SharedPreferences, routes to Dashboard or Login. |
| **Login** | Email/password → `AuthProvider.login`; on failure shows *"Wrong credentials. No half measures."* |
| **Dashboard** | Calls `EmpireProvider.loadState()` (`GET /game/state`); 2×2 `StatCard` grid (Purity, Dirty, Clean, DEA Heat) and navigation tiles. |
| **Cook Lab** | Effort slider (1–10, 9 divisions) → `EmpireProvider.cook(effort)`. |
| **DEA Status** | Shows the threat score/agent/evidence cached in `EmpireProvider` from the last `loadState()`; does not call `/dea/threat` itself. |
| **Legal** | Charge + evidence → `POST /legal/consult` with `player_context: "Mobile user"`. |

**Theme:** black scaffold, gold `#FFD700` primary, white body text. **Package/application id:** `com.heisenbergos.mobile`.

**Not yet on mobile:** registration, the laundering screen, and the transaction ledger (all available on web / via API).

---

## 11. API Reference

**Base URL:** `http://localhost:8080/api/v1`
**Content type:** `application/json`
**Auth:** every route except `/auth/*` requires `Authorization: Bearer <jwt>`.

### 11.1 Endpoint summary

| Method | Path | Auth | Body | Description |
|---|---|:-:|---|---|
| POST | `/auth/register` | ✗ | `username, email, password, alias?` | Create account |
| POST | `/auth/login` | ✗ | `email, password` | Returns JWT |
| GET | `/game/state` | ✓ | — | Empire + active DEA threat + legal cases |
| GET | `/empire` | ✓ | — | Empire (with front businesses); creates it if absent |
| POST | `/empire/cook` | ✓ | `effort` | Cook a batch |
| POST | `/empire/front` | ✓ | `name, type` | Add a front business |
| POST | `/launder` | ✓ | `amount, business_id` | Launder dirty money |
| GET | `/transactions` | ✓ | — | Latest 50 transactions |
| GET | `/dea/threat` | ✓ | — | Recompute, persist, and return threat |
| POST | `/legal/consult` | ✓ | `charge, evidence?, player_context?` | Ask Saul (Claude) |

### 11.2 Details & examples

> Timestamps (`CreatedAt`, `UpdatedAt`, `DeletedAt`) are trimmed from the examples for readability.

<details>
<summary><b>POST /auth/register</b> — 201 / 400 / 409</summary>

Validation: `username` required · `email` required + valid · `password` required, ≥ 6 chars · `alias` optional.

```jsonc
// 201
{ "success": true, "message": "Say my name",
  "data": { "ID": 1, "username": "walter", "email": "walt@example.com", "alias": "Heisenberg",
            "empire": { "ID": 0, "user_id": 0, "name": "", "purity_rating": 0, "…": "zero values" } } }
```
`400` for validation errors, `409` for any creation error (e.g. duplicate username/email). The `empire` object is a zero-valued placeholder at registration time; the real empire is created lazily.
</details>

<details>
<summary><b>POST /auth/login</b> — 200 / 400 / 401</summary>

```jsonc
// 200
{ "success": true, "message": "I am the one who knocks",
  "data": { "token": "eyJhbGciOiJIUzI1NiIs…" } }
// 401
{ "success": false, "message": "", "error": "invalid credentials" }   // or "user not found"
```
</details>

<details>
<summary><b>GET /game/state</b> — 200</summary>

```jsonc
{ "success": true, "message": "Game state loaded",
  "data": {
    "empire":     { "ID": 1, "name": "Empire", "purity_rating": 50, "batch_yield": 110,
                    "dirty_money": 50000, "clean_money": 75000, "heat_level": 101,
                    "territory_count": 0, "front_businesses": [ /* … */ ] },
    "dea_threat": { "ID": 1, "threat_score": 54.4, "evidence_count": 1,
                    "lead_agent": "Hank Schrader", "is_active": true },
    "legal_cases": []
  } }
```
`dea_threat` is a zero-valued object until `/dea/threat` has been called once.
</details>

<details>
<summary><b>POST /empire/cook</b> — 200 / 400</summary>

```jsonc
// Request
{ "effort": 7 }
// 200 — returns the updated empire
{ "success": true, "message": "Batch cooked",
  "data": { "ID": 1, "purity_rating": 50, "batch_yield": 27.5,
            "dirty_money": 26250, "heat_level": 17.5, "…": "…" } }
```
`effort` is `required` (zero is rejected). The UI restricts it to 1–10; the server does not enforce a range (see [§14](#14-implementation-status--known-limitations)).
</details>

<details>
<summary><b>POST /empire/front</b> — 201 / 400 / 500</summary>

```jsonc
// Request
{ "name": "Los Pollos Hermanos", "type": "restaurant" }
// 201
{ "success": true, "message": "Front business added — keep it clean",
  "data": { "ID": 1, "empire_id": 1, "name": "Los Pollos Hermanos", "type": "restaurant",
            "revenue": 0, "suspicion": 0, "is_active": true } }
```
Suggested `type` values (free text today): `restaurant`, `car_wash`, `nail_salon`, … — the ML laundering model gives `car_wash` and `nail_salon` special treatment.
</details>

<details>
<summary><b>POST /launder</b> — 200 / 400</summary>

```jsonc
// Request
{ "amount": 100000, "business_id": 1 }
// 200
{ "success": true, "message": "Money's clean, Mr. White",
  "data": { "ID": 1, "empire_id": 1, "amount": 75000, "type": "laundered",
            "description": "Los Pollos transfer", "business_id": 1, "flagged": true } }
// 400
{ "success": false, "message": "", "error": "insufficient dirty money" }
```
</details>

<details>
<summary><b>GET /transactions</b> — 200</summary>

Returns an array of up to 50 `Transaction` objects, newest first.
</details>

<details>
<summary><b>GET /dea/threat</b> — 200</summary>

```jsonc
{ "success": true, "message": "Stay one step ahead of Hank",
  "data": { "ID": 1, "empire_id": 1, "threat_score": 54.4, "evidence_count": 1,
            "lead_agent": "Hank Schrader", "is_active": true } }
```
Calling this endpoint **mutates state** (upserts the threat row).
</details>

<details>
<summary><b>POST /legal/consult</b> — 200 / 400 / 500</summary>

```jsonc
// Request
{ "charge": "Money laundering",
  "evidence": "Bank records showing structured deposits",
  "player_context": "Run a car wash, cash-heavy, first offense" }
// 200
{ "success": true, "message": "Better call Saul",
  "data": { "advice": "<Saul's in-character reply>", "risk": "medium",
            "defense": "See Saul's advice above" } }
// 500
{ "success": false, "message": "", "error": "Saul is unavailable right now" }
```
Requires `ANTHROPIC_API_KEY` on the backend.
</details>

### 11.3 Status code cheat-sheet

| Code | When |
|---:|---|
| 200 / 201 | Success / created |
| 400 | Validation or business-rule failure (bad JSON, `insufficient dirty money`, …) |
| 401 | Missing/invalid/expired token, or bad login credentials |
| 409 | Registration conflict |
| 500 | Server/DB error, or Saul unavailable |

### 11.4 ML service endpoints

| Method | Path | Body schema | Purpose |
|---|---|---|---|
| POST | `/ml/threat/analyze` | `ThreatInput` | Threat probability + risk band |
| POST | `/ml/yield/optimize` | `YieldInput` | Yield prediction + optimal effort |
| POST | `/ml/laundering/analyze` | `TransactionPattern` | Anomaly + suspicious pattern list |
| GET | `/health` | — | Liveness |
| GET | `/docs` · `/redoc` · `/openapi.json` | — | Auto-generated API docs |

Schemas (`app/models/schemas.py`):

```python
ThreatInput      { empire_id:int, heat_level:float, flagged_transactions:int,
                   batch_yield:float, territory_count:int, days_active:int }
ThreatOutput     { threat_score:float, risk_level:str, prediction:str, recommendation:str }
YieldInput       { current_purity:float, effort_level:float, supply_quality:float, lab_capacity:float }
YieldOutput      { predicted_yield:float, optimal_effort:float, purity_projection:float, efficiency_score:float }
TransactionPattern { amounts:List[float], business_revenues:List[float], time_gaps:List[float], business_type:str }
LaunderingRisk   { risk_score:float, anomaly_detected:bool, suspicious_patterns:List[str], recommendation:str }
```

---

## 12. Environment Variables

### Backend — `backend/.env`

| Variable | Required | Example | Description |
|---|:-:|---|---|
| `DATABASE_URL` | ✅ | `postgres://postgres:password@localhost:5432/heisenberg_os` | Postgres DSN. If it is wrong the server exits on boot. |
| `JWT_SECRET` | ✅ | *(long random string)* | HS256 signing key. **Change the sample value.** |
| `PORT` | — | `8080` | Listen port (default `8080`). |
| `ANTHROPIC_API_KEY` | for Saul | `sk-ant-…` | From [console.anthropic.com](https://console.anthropic.com). Without it `/legal/consult` fails. |
| `ML_SERVICE_URL` | reserved | `http://localhost:8000` | Placeholder for the planned backend → ML integration; **not read by the code yet**. |

```env
DATABASE_URL=postgres://postgres:password@localhost:5432/heisenberg_os
JWT_SECRET=replace-me-with-a-long-random-string
PORT=8080
ANTHROPIC_API_KEY=your_anthropic_api_key_here
ML_SERVICE_URL=http://localhost:8000
```

> Generate a strong secret: `openssl rand -hex 32`.
> The repository's `.gitignore` excludes `.env`; commit a `.env.example` with placeholder values instead.

### Web — `web/.env.local`

| Variable | Default | Description |
|---|---|---|
| `NEXT_PUBLIC_API_URL` | `http://localhost:8080/api/v1` | Backend base URL (inlined at build time). |

### ML service

No environment variables are required.

### Mobile

No env file — the base URL is a constant in `lib/services/api_service.dart`.

---

## 13. Security Notes

What the project already does well:

- Passwords are stored only as **bcrypt** hashes; `PasswordHash` is excluded from JSON (`json:"-"`).
- JWTs are signed (HS256), expire after 72 h, and are required on all game routes.
- The Anthropic key lives only on the server; clients never see it.
- Registration enforces unique usernames/emails at the database level and validates email format and password length.

Recommended hardening before any non-local deployment:

| Area | Recommendation |
|---|---|
| JWT secret | Use a long random `JWT_SECRET`; never reuse the sample value. |
| JWT parsing | Explicitly assert the signing method (`HS256`) inside the `jwt.Parse` keyfunc; use `jwt.WithValidMethods`. |
| Transport | Terminate TLS in front of the API; the web client and mobile app currently use plain HTTP in dev. |
| CORS | Replace the hard-coded `http://localhost:3000` with a configurable allow-list; restrict the ML service from `*`. |
| Token storage | The web app keeps the JWT in `localStorage` (XSS-exposed). Consider httpOnly cookies. |
| Input validation | Enforce numeric ranges server-side (see [§14](#14-implementation-status--known-limitations)). |
| Rate limiting | Add limits to `/auth/*` and `/legal/consult` (the latter costs money per call). |
| Gin mode | Run with `GIN_MODE=release`. |
| Network access | Do not expose the ML service publicly; keep it on a private network behind the Go API. |

---

## 14. Implementation Status & Known Limitations

An honest snapshot of the current codebase, so contributors know where to dig in.

### 14.1 Feature status

| Feature | Backend | ML | Web | Mobile |
|---|:-:|:-:|:-:|:-:|
| Register / login (JWT) | ✅ | — | ✅ | ✅ login only |
| Empire state | ✅ | — | ✅ | ✅ |
| Cook Lab | ✅ | 🟡 optimizer exists, not called | ✅ | ✅ |
| Front businesses | ✅ API | — | 🟡 no UI (laundering asks for a raw business ID) | ❌ |
| Laundering + ledger | ✅ | 🟡 detector exists, not called | ✅ | ❌ |
| DEA threat | ✅ heuristic formula | 🟡 model exists, not called | ✅ | ✅ |
| Saul Goodman | ✅ | — | ✅ | ✅ |
| Legal cases (persist/track) | 🟡 table only | — | ❌ | ❌ |
| Territories | 🟡 field only | — | 🟡 display only | ❌ |

✅ working · 🟡 partial/scaffolded · ❌ not implemented

### 14.2 Behaviors worth knowing

- **ML ↔ backend are decoupled.** The Go API never calls `/ml/*`. The in-Go threat formula and the ML threat model are independent implementations.
- **Server-side range validation is missing.** `effort` and `amount` only use `binding:"required"` (which rejects `0`). Negative values are accepted: a negative `amount` passes the funds check and would *increase* dirty money, reduce clean money, and lower heat. The UI restricts inputs, the API does not. Add `gt=0` / `min=1,max=10` binding tags.
- **Heat is clamped in cooking but not in laundering**, so `heat_level` can exceed 100 (the threat *score* itself is capped at 100). Heat never decays over time.
- **Purity never changes** (it stays at 50), so yield per effort point is constant (`2.5 kg × effort`).
- **No transactional safety.** Cook and launder are read-modify-write sequences without a DB transaction or row lock; concurrent requests can race.
- **`business_id` is not validated** against the player's front businesses, and the ledger description is always `"Los Pollos transfer"`.
- **Front business `revenue` and `suspicion`** are stored but nothing updates them yet.
- **Saul's `risk` and `defense` fields are placeholders**; only `advice` carries real content. The Anthropic response is parsed with unchecked type assertions (an upstream error body would panic, which Gin's recovery turns into a 500), and the HTTP client has no timeout. The model ID `claude-sonnet-4-20250514` is hard-coded.
- **Ignored errors.** Several DB calls (`GetOrCreate`, `Create`, `Save`) discard errors, and `empire/handler.go` still contains a few unused leftover variables in `cookBatch`.
- **The yield optimizer's objective is linear in effort**, so `optimal_effort` is always the upper bound (`100`) for positive inputs.
- **The laundering detector** ignores `business_revenues`; its IsolationForest baseline is synthetic exponential data, not real history.
- **The evidence graph** (`add_evidence_node`) exists in `dea_agent.py` but no endpoint exposes it.
- **Pydantic v2 deprecation:** `threat.py` uses `data.dict()`; prefer `data.model_dump()`.
- **Auth edge cases:** every registration error maps to HTTP 409; login distinguishes "user not found" from "invalid credentials" (user enumeration).
- **Web:** token guard exists only on `/dashboard`; `any` types are used for API payloads.
- **The "v2.0.1 — SECURE" label** in the dashboard header and landing-page stats are cosmetic strings.

### 14.3 Quick wins (good first issues)

1. Add `.env.example` and `docker-compose.yml` to the repo (see [§15](#15-docker-suggested-setup)).
2. Validate ranges with Gin binding tags and clamp heat in laundering.
3. Add a `ml` client package in Go and replace the heuristic in `dea.UpdateThreat` (config via `ML_SERVICE_URL`, with the formula as fallback).
4. Add a front-business list/selector to the web laundering page and a laundering screen on mobile.
5. Wrap cook/launder in `db.Transaction` with `SELECT … FOR UPDATE`.
6. Add the Android `INTERNET` permission and a configurable API base URL (`--dart-define`).
7. Re-encode `ml-service/requirements.txt` as UTF-8 and trim it to real dependencies.
8. Add unit tests (see [§17](#17-testing--code-quality)).

---

## 15. Docker (Suggested Setup)

The repository does not yet contain a `docker-compose.yml`. Here is a drop-in file for the database; extend it with `backend` / `ml-service` services once Dockerfiles exist.

<details>
<summary><b>docker-compose.yml (PostgreSQL only)</b></summary>

```yaml
services:
  postgres:
    image: postgres:15
    container_name: heisenberg-postgres
    restart: unless-stopped
    environment:
      POSTGRES_USER: postgres
      POSTGRES_PASSWORD: password
      POSTGRES_DB: heisenberg_os
    ports:
      - "5432:5432"
    volumes:
      - heisenberg_pgdata:/var/lib/postgresql/data
    healthcheck:
      test: ["CMD-SHELL", "pg_isready -U postgres -d heisenberg_os"]
      interval: 5s
      timeout: 3s
      retries: 10

volumes:
  heisenberg_pgdata:
```

```bash
docker compose up -d      # start
docker compose down       # stop (data persists in the named volume)
docker compose down -v    # stop AND wipe the database
```
</details>

---

## 16. Troubleshooting

| Symptom | Likely cause | Fix |
|---|---|---|
| Backend exits with `Failed to connect to database` | Postgres not running or wrong `DATABASE_URL` | `docker start heisenberg-postgres`; verify the DSN in `.env`. |
| Backend log says `No .env file found` and requests fail | `.env` missing | Create `backend/.env` ([§12](#12-environment-variables)). |
| Every protected call returns `401 invalid token` | `JWT_SECRET` changed since the token was issued, or token expired (72 h) | Log in again. |
| Browser shows a CORS error | Web app not served from `http://localhost:3000` | Use port 3000 or edit the allow-list in `cmd/server/main.go`. |
| Saul says *"Saul is unavailable right now"* | Missing/invalid `ANTHROPIC_API_KEY`, no network, or API error | Set a valid key; check backend logs. |
| Dashboard shows **DEA Heat 0 %** after cooking | Threat is computed on demand | Open **DEA Status** (calls `/dea/threat`), then return to the dashboard. |
| Laundering returns `insufficient dirty money` | Not enough dirty money | Cook first; each effort-10 cook yields $37,500. |
| Laundering "business ID" has no effect / unknown | No UI for fronts yet | `POST /empire/front` to create one and note its `ID`; the default in the UI is `1`. |
| `pip install -r requirements.txt` fails | Python < 3.11 (numpy 2.x / scikit-learn 1.8) or file encoding | Use Python 3.11+; re-save the file as UTF-8. |
| Mobile app can't reach the API on an emulator | Wrong base URL | Android emulator → `10.0.2.2`; iOS simulator → `localhost`. |
| Mobile app can't reach the API on a real phone | Using `localhost`/`10.0.2.2` | Use your computer's LAN IP; allow port 8080 through the firewall; same network. |
| Release APK never reaches the backend | Committed `android/app/src/main/AndroidManifest.xml` has no `INTERNET` permission (only the debug/profile manifests do) | Add `<uses-permission android:name="android.permission.INTERNET"/>` to the main manifest. |
| `Cleartext HTTP traffic not permitted` on Android | Plain HTTP blocked on newer Android versions | Add `android:usesCleartextTraffic="true"` to `<application>` for dev, or serve the API over HTTPS. |
| Next.js behaves differently from tutorials | Next 16 / React 19 have breaking changes | Read `web/AGENTS.md` and `node_modules/next/dist/docs/`. |

---

## 17. Testing & Code Quality

Current state:

| Area | Status |
|---|---|
| Backend (Go) | No test files yet |
| ML service | No test files yet |
| Web | ESLint configured (`npm run lint`, `eslint-config-next`) |
| Mobile | Default `test/widget_test.dart` + `flutter_lints` (`flutter test`, `flutter analyze`) |

Suggested starting points:

```bash
# Go — table-driven tests for Cook(), LaunderMoney(), threat formula
cd backend && go test ./...

# Python — FastAPI TestClient against the three /ml endpoints
cd ml-service && pip install pytest httpx && pytest

# Web / Mobile
cd web && npm run lint
cd mobile && flutter analyze && flutter test
```

High-value test cases: bcrypt round-trip and JWT expiry; `Cook` math at several purities; `LaunderMoney` insufficient-funds and the $50,000 flag boundary; threat-score clamping at 100; negative-input rejection (once validation is added); ML response schema and risk-band thresholds.

---

## 18. Git Workflow

Feature-branch workflow:

```
main
├── feat/backend-setup
├── feat/ml-service
├── feat/web-frontend
└── feat/mobile
```

**Commit convention** (Conventional Commits style):

```
feat(scope): description       # new feature
fix(scope): description        # bug fix
chore(scope): description      # tooling, deps
refactor(scope): description   # code change, no feature
```

<details>
<summary><b>Commit history of the project</b></summary>

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
</details>

---

## 19. Roadmap

```mermaid
timeline
    title Heisenberg OS roadmap
    Phase 1 - Foundation (done) : Go API with JWT auth : Cook, launder, DEA, Saul : FastAPI ML service : Next.js and Flutter clients
    Phase 2 - Multiplayer : WebSocket live empire updates : Multiplayer DEA mode (one player is Hank) : Leaderboard of biggest empires before capture
    Phase 3 - Advanced ML : Go backend calls FastAPI directly : Reinforcement-learning DEA agent : NLP wire-tap on player chat
    Phase 4 - Game depth : Territory map and control zones : Supplier and distributor graph : Seasons with new mechanics : Mobile push alerts for DEA
```

### Phase 2 — Multiplayer
- [ ] WebSocket support for real-time empire updates
- [ ] Multiplayer DEA mode — one player is Hank, others run the empire
- [ ] Leaderboard — biggest empire before getting caught

### Phase 3 — Advanced ML
- [ ] Wire the Go backend to call the FastAPI ML endpoints directly (`ML_SERVICE_URL`)
- [ ] Reinforcement-learning DEA agent that adapts to player patterns
- [ ] NLP-based wire-tap system — analyze player chat for suspicious keywords

### Phase 4 — Game Depth
- [ ] Territory map with visual control zones
- [ ] Supplier/distributor relationship graph
- [ ] Season system — each season introduces new mechanics
- [ ] Mobile push notifications for DEA alerts

### Engineering backlog (from [§14](#14-implementation-status--known-limitations))
- [ ] Server-side input validation & heat clamping
- [ ] DB transactions for cook/launder
- [ ] Persist and surface `legal_cases`
- [ ] Front-business management UI (web + mobile) and laundering on mobile
- [ ] Test suites for Go, Python, web, and mobile
- [ ] Dockerfiles + full `docker-compose.yml`
- [ ] CI (lint, test, build) on pull requests

---

## 20. License & Disclaimer

This project is for **educational and portfolio purposes only**. No real criminal activity is endorsed, facilitated, or simulated in a harmful way. All game mechanics are fictional abstractions of real-world systems, and all numbers, formulas, and "detection" models are toy implementations — they are **not** real financial-crime or law-enforcement tooling.

*Breaking Bad* and *Better Call Saul* are properties of their respective owners; this project is an unaffiliated fan-inspired homage.

---

<div align="center">

*"I am not in danger, Skyler. I am the danger."*

</div>
