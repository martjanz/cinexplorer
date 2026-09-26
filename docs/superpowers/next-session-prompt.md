# Prompt para continuar Cinexplorer en una sesión nueva

Copiá y pegá esto como primer mensaje:

---

Estoy retomando el proyecto **Cinexplorer** (repo: https://github.com/martjanz/cinexplorer). Esta sesión es para **implementar la Etapa 4a: Catálogo navegable**, que ya está diseñada y planificada.

Trabajá en el worktree `D:\x-projects\cinexplorer\.claude\worktrees\cinexplorer-etapa-4-interfaz-ac1b01`, rama `claude/cinexplorer-etapa-4-interfaz-ac1b01` (ya tiene el spec y el plan commiteados sobre `main`).

Leé antes de arrancar:
- `docs/superpowers/specs/2026-09-26-cinexplorer-m4a-catalogo-design.md` — spec de la Etapa 4a (Svelte + Vite embebido, Explorar con facetas en la URL, Ficha con versiones en tarjetas, Revisar: Sin identificar y Duplicados; pendientes de la Etapa 3 que se resuelven acá).
- `docs/superpowers/plans/2026-09-26-cinexplorer-m4a-catalogo.md` — plan de 18 tareas con el código completo. Se generó a partir de un prototipo probado y se validó aplicándolo sobre un árbol limpio: el resultado fue idéntico al prototipo (incluido el build embebido) y cada tarea quedó en verde. Leé la sección **Entorno** (Node ≥ 22.12: en esta máquina, `export PATH="/c/Users/martin/AppData/Roaming/nvm/v24.21.0:$PATH"` antes de `npm`).
- La rama local `m4a-proto` tiene el prototipo, un commit por tarea aproximadamente. Es solo de referencia (por ejemplo, para comparar si algo no coincide): **no la mergees**.

Corré `go test ./...` para confirmar que todo sigue en verde antes de tocar nada.

Flujo (el mismo de las etapas anteriores):
1. **superpowers:subagent-driven-development** sobre el plan: implementador barato (haiku), porque el plan trae el código completo, más revisor de spec y revisor de calidad por tarea, iterando hasta que cada revisión apruebe. Los issues "Important" se corrigen antes de avanzar. Revisión final de toda la etapa con un modelo fuerte.
2. Prueba real al final (Task 18, Step 3): binario con token sobre `D:\cine\1970s`, recorriendo Explorar, una Ficha, la cola de Revisar con atajos, Duplicados y el ancho de teléfono.
3. **superpowers:finishing-a-development-branch** y merge a `main` (fast-forward, sin PR). Push a GitHub al final, y verificá que el job `web` de la CI pase (comprueba que el build commiteado coincide con `web/`).

Después de la 4a viene la **Etapa 4b: Descubrimiento y primer uso** (Inicio estilo MUBI, búsqueda instantánea con FTS5, asistente de primer uso: raíces, token de TMDB, idioma; spec general §5.1, §5.6, §5.7). No hace falta planificarla en esta sesión.

---
