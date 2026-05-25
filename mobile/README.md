# Heisenberg OS — Mobile

Flutter mobile client for the Heisenberg OS empire simulation platform.

## Requirements

- Flutter SDK 3.x+
- Dart 3.x+
- Android Studio / Xcode for emulators

## Setup

```bash
flutter pub get
flutter run
```

## Environment

Update the base URL in `lib/services/api_service.dart`:

- Android emulator: `http://10.0.2.2:8080/api/v1`
- iOS simulator: `http://localhost:8080/api/v1`
- Physical device: `http://YOUR_LOCAL_IP:8080/api/v1`

## Screens

- Splash — auth check on launch
- Login — JWT authentication
- Dashboard — empire stats overview
- Cook Lab — batch synthesis controls
- DEA Status — live threat monitoring
- Legal — Saul Goodman AI consultation

## Dependencies

- `provider` — state management
- `http` — API calls
- `shared_preferences` — token storage