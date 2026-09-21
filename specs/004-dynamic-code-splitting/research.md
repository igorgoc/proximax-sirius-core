# Research: Code-Splitting Patterns with Named Exports in React 18

## Named Export Lazy-Loading Pattern
Since the components in `frontend/src/components/*.tsx` use named exports (e.g. `export function ConfigTab(...)`), `React.lazy()` is configured with:
```typescript
const ConfigTab = lazy(() => import('./components/ConfigTab').then(m => ({ default: m.ConfigTab })));
```

## Suspense Placement
Wrapping the `<main>` tab router in `<Suspense fallback={<TabLoadingSpinner />}>` ensures smooth rendering while retaining the static `Header`, `NavigationTabs`, and `StatusBar` across transitions.
