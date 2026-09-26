# Prompt para continuar Cinexplorer en una sesión nueva

Copiá y pegá esto como primer mensaje:

---

Estoy retomando el proyecto **Cinexplorer** en `D:\x-projects\cinexplorer` (repo: https://github.com/martjanz/cinexplorer, rama `main`).

Contexto que ya existe y hay que leer antes de arrancar:
- `docs/superpowers/specs/2026-09-25-cinexplorer-design.md` — diseño completo aprobado (arquitectura, modelo de datos, pipeline, UI, las 5 etapas).
- `docs/superpowers/plans/2026-09-25-cinexplorer-m1-nucleo.md` — plan de la Etapa 1 (núcleo), implementada y en `main`.
- `docs/superpowers/specs/2026-09-25-cinexplorer-m2-datos-tecnicos-design.md` y `docs/superpowers/plans/2026-09-25-cinexplorer-m2-datos-tecnicos.md` — Etapa 2 (datos técnicos: lectores MKV/MP4/AVI/IFO, fallback ffprobe, tabla `media`, mejor versión), implementada y en `main`. La sección 10 del spec lista pendientes a tener en cuenta (en particular: `markBest` agrupa por `quality.GroupKey` provisorio, que la Etapa 3 reemplaza por el id de TMDB). Corré `go test ./...` para confirmar que sigue todo verde antes de tocar nada.

Quiero seguir con la **Etapa 3: Identificación** de la hoja de ruta: cliente TMDB con rate limit y reintentos, Wikidata, puntaje de confianza, tabla de películas, identificaciones y correcciones atadas a la huella, IMDb desde `.nfo`, caché de imágenes, reintentos sin red. Referencia en el spec general: §3.1, §4 pasos 5–6, §4.2.

Flujo a seguir (mismo que usamos para las Etapas 1 y 2):
1. **superpowers:brainstorming** para refinar el alcance exacto de la Etapa 3 si hace falta (probablemente ya está bastante definido en el spec, así que puede ser corto) — dado que ya tenemos diseño aprobado, confirmá conmigo el alcance y pasá directo a planificar si no hay ambigüedad real.
2. **superpowers:writing-plans** para escribir `docs/superpowers/plans/YYYY-MM-DD-cinexplorer-m3-<algo>.md` con el mismo nivel de detalle que el plan de la Etapa 1 (TDD, código completo en cada paso, comandos exactos).
3. **superpowers:using-git-worktrees** para aislar el trabajo en `.worktrees/etapa3-<algo>` con una rama nueva.
4. **superpowers:subagent-driven-development** para ejecutar tarea por tarea: un subagente implementador (modelo barato: haiku, porque el plan trae el código completo) + revisor de spec + revisor de calidad por tarea, iterando hasta que cada revisión apruebe antes de pasar a la siguiente. Los issues "Important" que encuentren los revisores se corrigen antes de avanzar, no se acumulan.
5. Al terminar todas las tareas, **superpowers:finishing-a-development-branch** y merge a `main` (fast-forward, sin PR — no hay necesidad de review humano intermedio en este proyecto solo mío).
6. Push a GitHub al final.

Después de la Etapa 3 quedan pendientes, en orden: Etapa 4 (interfaz Svelte real: Inicio/Explorar/Ficha/Revisar/búsqueda), Etapa 5 (curaduría: listas, etiquetas, importación de `Collections/`). No hace falta planificarlas ahora.

---
