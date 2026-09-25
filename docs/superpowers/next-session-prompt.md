# Prompt para continuar Cinexplorer en una sesión nueva

Copiá y pegá esto como primer mensaje:

---

Estoy retomando el proyecto **Cinexplorer** en `D:\x-projects\cinexplorer` (repo: https://github.com/martjanz/cinexplorer, rama `main`).

Contexto que ya existe y hay que leer antes de arrancar:
- `docs/superpowers/specs/2026-09-25-cinexplorer-design.md` — diseño completo aprobado (arquitectura, modelo de datos, pipeline, UI, las 5 etapas).
- `docs/superpowers/plans/2026-09-25-cinexplorer-m1-nucleo.md` — plan de la Etapa 1, ya implementada y mergeada a `main` (escaneo incremental de solo lectura, parser de nombres, agrupamiento en versiones, SQLite, servidor HTTP local mínimo, build multiplataforma, CI). Corré `go test ./...` para confirmar que sigue todo verde antes de tocar nada.

Quiero seguir con la **Etapa 2: Datos técnicos** de la hoja de ruta (tabla en la sección "Hoja de ruta (etapas)" del plan de la Etapa 1): lectores nativos de headers MKV/MP4/AVI (resolución, codecs, duración, pistas de audio/subs), fallback opcional a `ffprobe` si está instalado, y el criterio de "mejor versión" para la ficha de película. Referencia en el spec: §4 paso 4, §5.3.

Flujo a seguir (mismo que usamos para la Etapa 1):
1. **superpowers:brainstorming** para refinar el alcance exacto de la Etapa 2 si hace falta (probablemente ya está bastante definido en el spec, así que puede ser corto) — dado que ya tenemos diseño aprobado, confirmá conmigo el alcance y pasá directo a planificar si no hay ambigüedad real.
2. **superpowers:writing-plans** para escribir `docs/superpowers/plans/YYYY-MM-DD-cinexplorer-m2-<algo>.md` con el mismo nivel de detalle que el plan de la Etapa 1 (TDD, código completo en cada paso, comandos exactos).
3. **superpowers:using-git-worktrees** para aislar el trabajo en `.worktrees/etapa2-<algo>` con una rama nueva.
4. **superpowers:subagent-driven-development** para ejecutar tarea por tarea: un subagente implementador + revisor de spec + revisor de calidad por tarea, iterando hasta que cada revisión apruebe antes de pasar a la siguiente. Los issues "Important" que encuentren los revisores se corrigen antes de avanzar, no se acumulan.
5. Al terminar todas las tareas, **superpowers:finishing-a-development-branch** y merge a `main` (fast-forward, sin PR — no hay necesidad de review humano intermedio en este proyecto solo mío).
6. Push a GitHub al final.

Después de la Etapa 2 quedan pendientes, en orden: Etapa 3 (TMDB + Wikidata + identificación), Etapa 4 (interfaz Svelte real: Inicio/Explorar/Ficha/Revisar/búsqueda), Etapa 5 (curaduría: listas, etiquetas, importación de `Collections/`). No hace falta planificarlas ahora, solo tenerlas en mente como lo que sigue.

---
