# Cinexplorer — Identificación: lecturas alternativas del nombre — Diseño

Fecha: 2026-10-21
Estado: borrador para aprobación
Spec de referencia: `2026-09-26-cinexplorer-m3-identificacion-design.md` (§6 búsqueda y puntaje)

## 1. Por qué

En la colección real (`D:\cinexplorer-e2e-tmp`, 1736 identificaciones) quedan **396 sin identificar (~23 %)**. Revisarlas hoy es lento: en 280 de ellas TMDB no devolvió nada, la lista de candidatos está vacía y hay que escribir o pegar el título a mano en el buscador, una por una.

Desglose de las 396:

| Grupo | Cant. | Ejemplos |
|---|---|---|
| Sin resultados, "Título - Director" | ~130 | `Azor - Andreas Fontana` (2021), `El campo - Hernán Belón`, `El Crazy Che - Pablo Chehebar y Nicolás Iacouzzi` |
| Sin resultados, "Director - Título" | varios | `Jean-Luc Godard - A Bout de souffle`, `Fernando Solanas - La hora de los hornos`, `C.Chabrol-Madame Bovary` |
| Sin resultados, director pegado al final sin guion | varios | `Mediterraneo G Salvatore`, `Loulou Pialat`, `Melancholia, Lars Von Trier`, `L'ivresse du pouvoir,C Chabrol` |
| Sin resultados, nombre mal partido por el parser (dos puntos) | pocos | título `Sebastián Díaz`, director `muerte y profanación 4 lonkos： Vida` |
| Sin resultados, episodios de TV | ~60 | `Veronica Mars S01E05` — **fuera de alcance**: se van a sacar de la colección |
| Empate entre películas homónimas | 39 | `Phoenix (2014)`, `Judy (2019)`, `Rio Bravo`, `El hombre de al lado` |
| Cerca pero no alcanza (errores de tipeo, año corrido, título en otro idioma) | 77 | `Children of the men`, `No sos vos, soy yo` 2006 vs 2004 |

## 2. Objetivo

Que la cola de revisión llegue con **la película correcta entre los candidatos, idealmente primera**, para resolver con un clic en vez de buscar a mano.

- **Objetivo principal:** que la mayoría de los "sin resultados" no‑TV pasen a tener candidatos útiles.
- **Secundario:** que una lectura alternativa, cuando título, año y director coinciden, se asigne automáticamente como hoy.
- **Anti‑objetivo:** asignaciones automáticas equivocadas. El usuario prefiere que una duda quede en revisión. Ante la duda, no se asigna.
- **No objetivos:** episodios de TV; cambiar la pantalla de revisión; cambiar `nameparse` (el título mostrado de una versión sin identificar sigue siendo el del nombre); el grupo "cerca pero no alcanza", salvo lo que mejore de rebote.

## 3. Decisiones

### 3.1 Lecturas alternativas (en el matcher, no en el parser)

Cuando la búsqueda actual (título, variantes entre corchetes, inglés y carpeta) **no es concluyente**, el matcher genera lecturas alternativas de la `Query` y busca con cada una, en orden, hasta que una sea concluyente o se agoten:

1. **Partir en el guion** (` - ` o ` – `; si no hay ninguno, `-` sin espacios entre letras cuando del lado izquierdo hay 1–3 palabras y del derecho 2 o más, como en `C.Chabrol-Madame Bovary` o `Maurice Pialat-Nous ne vieillirons pas ensemble`; así `Spider-Man`, `X-Men` y `Jean-Luc` no se parten). Un lado formado solo por dígitos no se usa como título ni como director:
   a. izquierda = título, derecha = director;
   b. izquierda = director, derecha = título.
2. **Partir en la última coma** (`Melancholia, Lars Von Trier`): izquierda = título, derecha = director.
3. **Quitar palabras del final** (`Mediterraneo G Salvatore`, `Loulou Pialat`): quitar 1 y luego 2 palabras finales; lo quitado pasa a ser el director. Solo si quedan al menos 1 palabra de título y el título original tiene 3 palabras o más (o 2 si la segunda tiene mayúscula inicial).
4. **Título y director cruzados**: cuando la query trae director, probar el director como título, con el título como director (caso `Sebastián Díaz` / `… Vida`). Prueba barata: una búsqueda.

Cada lectura es una `Query` normal (título, año de la original, director de la lectura o el original si la lectura no aporta uno) y se puntúa con el mismo `score` + verificación de director de hoy. Las lecturas que dan un título vacío o idéntico a uno ya buscado se descartan.

Los resultados de la query principal y de todas sus lecturas se juntan (sin duplicados por id de TMDB). Cada candidato se queda con el mejor puntaje que obtuvo bajo cualquier lectura **y recuerda de qué lectura vino ese puntaje** (la principal o una alternativa). Se guardan los 5 mejores, como hoy.

**Escala común (decisión de diseño):** el director de una lectura alternativa es una suposición, así que su puntaje se calcula como el de una query sin director (título + año reescalado a 0..1, igual que hoy) y, si TMDB confirma el director, como el de una query con director (título + año + 0,15). Las dos escalas tienen tope 1,0 y son las que ya conviven hoy entre queries con y sin director. La query principal se puntúa exactamente como hoy.

**Cuándo se corta el recorrido (decisión de diseño):** después de cada lectura se evalúa la lista juntada con la regla completa de §3.2 (incluida la condición del director). Si ya asignaría automáticamente, se corta; si no, sigue con la próxima lectura.

**Carpeta (decisión de diseño):** el paso de la carpeta (`Query.Folder`) sigue separado como hoy: se identifica la query principal con sus lecturas y, si no quedó automática, la carpeta, y se queda el mejor de los dos resultados. La carpeta no se mezcla con la lista de la principal. Si la carpeta también genera lecturas queda a criterio de quien implemente (§5).

### 3.2 Cuándo se asigna automáticamente

Igual que hoy (puntaje ≥ 0,80 y ventaja ≥ 0,10 sobre el segundo, sobre la lista final), con una condición extra: **un candidato cuyo mejor puntaje viene de una lectura alternativa se asigna automáticamente únicamente si TMDB confirma el director de esa lectura** (aunque también haya aparecido, más abajo, en la búsqueda principal). `Azor - Andreas Fontana` (título + año + director Fontana confirmado) se asigna solo; `Mediterraneo G Salvatore` se asigna solo si TMDB dice que el director es Salvatore. Sin director confirmado, va a revisión aunque título y año coincidan: una lectura inventada por el matcher nunca asigna sola.

### 3.3 Duración como desempate (solo para ordenar)

La duración del archivo ya está guardada (`media.duration_ms`, 1881 de 1886 archivos) y la de TMDB está en `Details.Runtime`.

- La `Query` recibe la duración total de la versión (suma de las partes principales; DVD: lo que ya calcula el análisis técnico para la versión).
- Cuando la lista final **no** es concluyente y hay duración, se piden los detalles de los mejores candidatos (hasta 5; se reutilizan los ya pedidos para el director y quedan para el enriquecimiento como hoy) y se ordena poniendo primero los que coinciden en duración (±max(5 min, 8 %)).
- La duración **no suma puntaje ni habilita asignación automática**: solo reordena lo que va a revisión. Esto resuelve los empates `Phoenix (2014)` / `Judy (2019)` dejando la correcta primera.

### 3.4 Versión del matcher

`MatcherVersion` pasa a 5, así se recalculan las identificaciones automáticas y sin identificar existentes. Las automáticas de hoy siguen igual (las lecturas alternativas solo corren si lo actual no es concluyente); el costo es volver a hacer su primera búsqueda.

### 3.5 Costo en pedidos a TMDB

El límite es 20 pedidos/s. Por cada versión no concluyente se agregan, como máximo, unas 6 búsquedas (cada una puede duplicarse sin año) y hasta 5 pedidos de detalles. Con ~400 dudosas son ~5000 pedidos extra en el peor caso, unos 4–5 minutos, una sola vez gracias a que el resultado queda guardado con su `query`.

## 4. Pruebas

- **Corpus** (`internal/identify/corpus_test.go`): agregar nombres reales de cada grupo con el resultado esperado y regrabar con `CINEXPLORER_TMDB_RECORD=1`. Como mínimo: `Azor - Andreas Fontana (2021)`, `El campo - Hernán Belón (2011)`, `Jean-Luc Godard - A Bout de souffle`, `Mediterraneo G Salvatore (1991)`, `Melancholia, Lars Von Trier (2011)`, y un empate (`Phoenix (2014)`) cuyo esperado es "a revisión" con la correcta primera. Los casos existentes deben seguir dando lo mismo.
- **Unitarias** para la generación de lecturas (qué lecturas salen de cada forma de nombre y cuáles no: `Spider-Man`, `X-Men` y `Jean-Luc` no se parten; `300 - Rise of an Empire` no da lectura con `300` como director ni como título).
- **Unitarias** para la condición de §3.2: un candidato que aparece flojo en la búsqueda principal y llega a ≥ 0,80 por una lectura alternativa sin director confirmado queda en revisión; con director confirmado, se asigna.
- **Medición final**: correr la identificación sobre una copia de `D:\cinexplorer-e2e-tmp` y reportar antes/después: automáticas, sin identificar con candidatos vacíos, y en cuántas de una muestra revisada a mano la correcta aparece primera.

## 5. Queda a criterio de quien implemente

- Dónde vive la generación de lecturas (archivo nuevo en `internal/identify` sugerido) y cómo se pasa la duración desde `store.IdentifyTarget`.
- El orden fino y los topes de §3.1 si la medición muestra que conviene otro.
- Si la lectura de la carpeta (`Query.Folder`) también genera lecturas alternativas.
