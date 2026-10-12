# NearBuy frontend

React 19 + TypeScript + Vite, Material UI, React Router, React Hook Form + Zod, Zustand, Axios.

## Run

The backend must be up first (`make up` from the repo root; the gateway is served by the kind ingress on `http://localhost/api`).

```sh
npm install
npm run dev          # http://localhost:5173
```

Vite proxies `/api` to `http://localhost` because the gateway has no CORS. Point it elsewhere with `API_TARGET=http://host:port npm run dev`.

## Layout

```
src/
  entities/   API wire types (order ids are strings: they exceed Number.MAX_SAFE_INTEGER)
  services/   api-client (axios, big-int-safe JSON, bearer token, 401 -> logout), error messages
  stores/     authStore (zustand, persisted)
  hooks/      useData + resource hooks, useOrder (polls the 202 -> 404 -> 200 seckill flow)
  components/ NavBar, cards, forms, PrivateRoutes
  pages/      one per route (see routes.tsx)
```
