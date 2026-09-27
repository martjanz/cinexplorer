# Prompt para continuar Cinexplorer en una sesión nueva

Copiá y pegá esto como primer mensaje:

---

Estoy retomando el proyecto **Cinexplorer** (repo: https://github.com/martjanz/cinexplorer). Las etapas 1 a 4a están en `main` (la 4a: interfaz Svelte embebida con Explorar, Ficha y Revisar). Esta sesión es para **diseñar y planificar la Etapa 4b: Descubrimiento y primer uso**: Inicio estilo MUBI, búsqueda instantánea con FTS5 y asistente de primer uso (raíces, token de TMDB, idioma).

Leé antes de arrancar:
- `docs/superpowers/specs/2026-09-25-cinexplorer-design.md` — spec general, en especial §5.1 (Inicio), §5.6 (búsqueda) y §5.7 (primer uso).
- `docs/superpowers/specs/2026-09-26-cinexplorer-m4a-catalogo-design.md` — spec de la 4a; §11 tiene los pendientes, incluidos los menores de la revisión final.
- `docs/superpowers/plans/2026-09-26-cinexplorer-m4a-catalogo.md` — el plan de la 4a, como referencia de formato (código completo por tarea, generado desde un prototipo probado).
- `README.md` — cómo se compila y se prueba; el frontend necesita Node ≥ 22.12 (en esta máquina, `export PATH="/c/Users/martin/AppData/Roaming/nvm/v24.21.0:$PATH"` antes de `npm`).

Corré `go test ./...` y `cd web && npm test` para confirmar que todo sigue en verde.

Flujo (el mismo de las etapas anteriores):
1. **superpowers:brainstorming** para el diseño de la 4b y spec en `docs/superpowers/specs/`.
2. Prototipo probado (incluida una prueba real con token sobre `D:\cine\1970s`) y **superpowers:writing-plans** con el código completo, validado aplicándolo sobre un árbol limpio.
3. La implementación va en una sesión aparte (subagent-driven-development, implementador haiku, revisores de spec y de calidad por tarea).

---
