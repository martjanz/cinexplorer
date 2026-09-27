# Prompt para continuar Cinexplorer en una sesión nueva

Copiá y pegá esto como primer mensaje:

---

Estoy retomando el proyecto **Cinexplorer** (repo: https://github.com/martjanz/cinexplorer). Las etapas 1 a 4a están en `main`. La Etapa 4b (Descubrimiento y primer uso: Inicio estilo MUBI, búsqueda instantánea con FTS5, asistente de primer uso, Ajustes e idioma es-AR) ya tiene spec y plan, en la rama `claude/adoring-curie-njeqf9` (si todavía no está mergeada a `main`, partí de esa rama). Esta sesión es para **implementarla**.

Leé antes de arrancar:
- `docs/superpowers/specs/2026-09-27-cinexplorer-m4b-descubrimiento-design.md` — spec de la 4b.
- `docs/superpowers/plans/2026-09-27-cinexplorer-m4b-descubrimiento.md` — el plan: 16 tareas con el código completo, generado desde un prototipo probado y validado aplicándolo sobre un árbol limpio.
- `README.md` — cómo se compila y se prueba; el frontend necesita Node ≥ 22.12 (en esta máquina, `export PATH="/c/Users/martin/AppData/Roaming/nvm/v24.21.0:$PATH"` antes de `npm`).

Corré `go test ./...` y `cd web && npm test` para confirmar que todo sigue en verde.

Flujo: **superpowers:subagent-driven-development** sobre el plan (implementador haiku, revisores de spec y de calidad por tarea). El código del plan se copia tal cual; si algo no compila o un test no da lo esperado, es un error del plan: se reporta en lugar de improvisar.

Pendiente de la sesión de diseño: el prototipo no pudo probarse contra TMDB (sin red). La **prueba real** de la Task 16 (token real, traducciones es-AR sobre películas no argentinas, forma de `/movie/{id}/translations`) es la primera vez que eso se comprueba: si falla, corregir antes del merge.

---
