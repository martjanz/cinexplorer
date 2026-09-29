# Prompt para continuar Cinexplorer en una sesión nueva

Copiá y pegá esto como primer mensaje:

---

Estoy retomando el proyecto **Cinexplorer** (repo: https://github.com/martjanz/cinexplorer). Las etapas 1 a 4b están en `main` (publicadas como v1.0.0), y también las películas en partes (spec y plan del 2026-09-28). Falta la **Etapa 5, curaduría**: listas y etiquetas propias, e importación de carpetas `Collections/` como listas. Todavía no tiene spec ni plan; esta sesión es para **diseñarla**.

Leé antes de arrancar:
- `docs/superpowers/specs/2026-09-25-cinexplorer-design.md` — spec general; la Etapa 5 sale de §1 (alcance), §4.1 (importación de `Collections/`), §5.2 (facetas Lista y Etiqueta), §5.3 (chips con "+" en la ficha), §5.4 (Listas) y §5.5 (pestaña Colecciones en Revisar).
- `docs/superpowers/specs/2026-09-28-cinexplorer-partes-design.md` — el antecedente más cercano de datos del usuario guardados aparte de las versiones (por huella), que sobreviven a los reescaneos.
- `README.md` — cómo se compila y se prueba.

Corré `go test ./...` y `cd web && npm test` para confirmar que todo sigue en verde.

Flujo: **superpowers:brainstorming** → spec en `docs/superpowers/specs/` → **superpowers:writing-plans** → plan en `docs/superpowers/plans/`.

---
