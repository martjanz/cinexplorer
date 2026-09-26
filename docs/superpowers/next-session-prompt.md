# Prompt para continuar Cinexplorer en una sesión nueva

Copiá y pegá esto como primer mensaje:

---

Estoy retomando el proyecto **Cinexplorer** en `D:\x-projects\cinexplorer` (repo: https://github.com/martjanz/cinexplorer, rama `main`).

Contexto que ya existe y hay que leer antes de arrancar:
- `docs/superpowers/specs/2026-09-25-cinexplorer-design.md` — diseño completo aprobado (arquitectura, modelo de datos, pipeline, UI, las 5 etapas).
- `docs/superpowers/plans/2026-09-25-cinexplorer-m1-nucleo.md` — plan de la Etapa 1 (núcleo), implementada y en `main`.
- `docs/superpowers/specs/2026-09-25-cinexplorer-m2-datos-tecnicos-design.md` y su plan — Etapa 2 (datos técnicos), implementada y en `main`.
- `docs/superpowers/specs/2026-09-26-cinexplorer-m3-identificacion-design.md` y `docs/superpowers/plans/2026-09-26-cinexplorer-m3-identificacion.md` — Etapa 3 (identificación: TMDB + Wikidata, puntaje de confianza, tablas `movies`/`identifications`, correcciones por huella, caché de imágenes, Runner en segundo plano con reintentos sin red, API de correcciones), implementada y en `main`. La sección 13 del spec lista pendientes; el primero a resolver al arrancar la Etapa 4 es que las imágenes cacheadas quedan viejas si cambia su ruta en TMDB (por ejemplo, al cambiar el idioma). También queda pendiente de la Etapa 2 envolver `Versions()` en una transacción de lectura. Corré `go test ./...` para confirmar que sigue todo verde antes de tocar nada.

Para la identificación hace falta un token de lectura de TMDB (v4) en `config.json` (`tmdbToken`); para regrabar el corpus de calibración (`CINEXPLORER_TMDB_RECORD=1 go test ./internal/identify -run Corpus -v`), en `CINEXPLORER_TMDB_TOKEN` o `~/.cinexplorer-tmdb-token`.

Quiero seguir con la **Etapa 4: Interfaz** de la hoja de ruta: Svelte + Vite embebido con `go:embed`, Inicio (estilo MUBI), Explorar con facetas en la URL (estilo Letterboxd), Ficha de película con versiones en tarjetas, Revisar (Sin identificar con candidatos, búsqueda manual, "no es una película", "es un extra de…"; Duplicados), búsqueda instantánea con FTS5, asistente de primer uso (raíces, token de TMDB, idioma). Referencia en el spec general: §5 completo. La API de la Etapa 3 ya soporta las acciones de Revisar.

Flujo a seguir (el mismo de las etapas anteriores):
1. **superpowers:brainstorming** para acotar el alcance de la Etapa 4. Esta etapa tiene más decisiones abiertas que las anteriores (estructura del frontend, build de Svelte dentro del flujo de Go, qué endpoints nuevos hacen falta para facetas y búsqueda, cuánto de §5 entra), así que consultame cada decisión.
2. **superpowers:writing-plans** para `docs/superpowers/plans/YYYY-MM-DD-cinexplorer-m4-<algo>.md`, con el mismo nivel de detalle que los planes anteriores (TDD, código completo en cada paso, comandos exactos). Como en la Etapa 3, conviene prototipar y verificar el código antes de escribir el plan, y validar el plan reproduciéndolo sobre un árbol limpio.
3. Trabajar en un worktree aislado con una rama nueva.
4. **superpowers:subagent-driven-development**: implementador barato (haiku) cuando el plan trae el código completo, más revisor de spec y revisor de calidad por tarea, iterando hasta que cada revisión apruebe. Los issues "Important" se corrigen antes de avanzar. Revisión final de toda la etapa con un modelo fuerte.
5. Al terminar, **superpowers:finishing-a-development-branch** y merge a `main` (fast-forward, sin PR).
6. Push a GitHub al final.

Después de la Etapa 4 queda la Etapa 5 (curaduría: listas, etiquetas, importación de `Collections/`). No hace falta planificarla ahora.

---
