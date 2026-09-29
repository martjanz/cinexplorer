# Cinexplorer — Etapa 5: Curaduría — Diseño

Fecha: 2026-09-28
Estado: aprobado en la conversación; pendiente de revisión del texto
Spec general: `2026-09-25-cinexplorer-design.md` (§1, §4.1, §5.2–§5.5)
Relacionado: `2026-09-28-cinexplorer-partes-design.md` (datos del usuario guardados por huella)

## 1. Objetivo

Listas propias del usuario y la importación de carpetas `Collections/` como listas.

Decisiones tomadas en el diseño:

- **Solo listas, sin etiquetas.** El spec general preveía las dos; se solapan casi por completo. Si las listas quedan pesadas, las etiquetas pueden venir después.
- **Una lista contiene películas identificadas y también contenido sin identificar** (por huella). Cuando ese contenido se identifica, se muestra como la película.
- **Importar una carpeta es una copia única:** la lista que resulta es una lista común, editable. Si después aparecen archivos nuevos en la carpeta, se ofrecen como agregado a esa lista.
- **Sin orden manual:** una lista se ordena como Explorar (año, título, agregado, tamaño) más "agregado a la lista".
- **Una lista es una faceta de Explorar** (`lista`): abrir una lista es `/explorar?lista=<id>`, con la grilla, el orden, los contadores, las demás facetas y las URLs enlazables que ya existen.

Fuera de alcance: etiquetas, orden manual de las listas o de sus entradas, listas en la búsqueda, listas sincronizadas con una carpeta.

## 2. Datos

Tablas nuevas en `internal/store/schema.sql`. Son datos del usuario: un reescaneo no las toca (como `identifications` y `part_links`).

```sql
-- Listas del usuario.
CREATE TABLE IF NOT EXISTS lists (
  id         INTEGER PRIMARY KEY,
  name       TEXT    NOT NULL UNIQUE COLLATE NOCASE,
  created_at INTEGER NOT NULL,          -- unix milliseconds
  updated_at INTEGER NOT NULL           -- último cambio de nombre o de entradas
);

-- Qué contiene cada lista: "movie:<tmdb id>" o "fp:<huella>".
CREATE TABLE IF NOT EXISTS list_entries (
  list_id  INTEGER NOT NULL REFERENCES lists(id) ON DELETE CASCADE,
  ref      TEXT    NOT NULL,
  added_at INTEGER NOT NULL,            -- unix milliseconds
  PRIMARY KEY (list_id, ref)
);

-- Carpetas de Collections/ sobre las que el usuario ya decidió.
CREATE TABLE IF NOT EXISTS collection_folders (
  path       TEXT    PRIMARY KEY,       -- relativa al directorio de la app, con '/'
  status     TEXT    NOT NULL,          -- imported | dismissed
  list_id    INTEGER REFERENCES lists(id) ON DELETE SET NULL,
  decided_at INTEGER NOT NULL           -- unix milliseconds
);

-- Contenido de cada carpeta que ya se ofreció (importado o descartado).
CREATE TABLE IF NOT EXISTS collection_seen (
  path        TEXT NOT NULL,
  fingerprint TEXT NOT NULL,
  PRIMARY KEY (path, fingerprint)
);
```

Reglas:

- **Agregar desde la ficha** de una película guarda `movie:<id>`; desde una versión sin identificar, `fp:<huella>`.
- **Importar una carpeta** guarda siempre `fp:<huella>`, una por versión (la huella del archivo representante). Así, si después se corrige la identificación de un archivo, la lista muestra la película corregida.
- **Agregar una entrada que ya está** no cambia nada (ni `added_at`).
- **Contenido que ya no está en el disco:** la entrada se conserva pero no se muestra ni se cuenta.
- **Quitar un ítem de una lista** borra todas las entradas que llevan a ese ítem: la `movie:` y las `fp:` de sus versiones.
- **Borrar una lista** borra sus entradas (cascada). Una carpeta importada a esa lista conserva `status = imported` con `list_id` nulo, y no se vuelve a ofrecer.
- **Novedades de una carpeta:** las huellas de la carpeta que no están en `collection_seen`. Importar o descartar marca como vistas todas las que la carpeta tiene en ese momento. Una carpeta importada con novedades se vuelve a ofrecer; una descartada, nunca. Una película que el usuario quitó de una lista importada ya está vista, así que no reaparece.
- `updated_at` cambia al renombrar, agregar o quitar entradas, y al importar novedades.

## 3. Catálogo

Todo en `internal/catalog`, sin SQL, como el resto del paquete.

### 3.1 Snapshot

`store.Snapshot` suma:

- `Lists []List` — cada lista con `ID`, `Name`, `CreatedAt`, `UpdatedAt` y sus entradas (`Ref`, `AddedAt`).
- `CollectionFolders map[string]CollectionDecision` — por ruta: `Status`, `ListID` y el conjunto de huellas vistas.

### 3.2 Resolución de entradas

Cada entrada se resuelve contra los ítems de Explorar (`build`):

- `movie:<id>` → el ítem de esa película.
- `fp:<huella>` según su identificación:
  - `auto` o `manual` con la película guardada → el ítem de la película;
  - `ignored` o `extra` → nada (oculta);
  - cualquier otro caso → el ítem sin identificar de esa huella.
- Un ítem al que llegan dos entradas aparece una vez; su "agregado a la lista" es el `added_at` más antiguo.
- Una entrada que no lleva a ningún ítem presente no se muestra.

### 3.3 Faceta `lista`

- `FacetList = "lista"`, valor = id de la lista (número). Se agrega a `FacetNames` después de `FacetCollection`. Sus valores en la barra son los nombres de las listas con contadores.
- Un id que no existe es un valor no válido: se ignora, igual que en las otras facetas.
- Orden nuevo `OrderListAdded = "agregado-lista"`, solo válido con la faceta `lista` aplicada; es el orden por defecto en ese caso (más reciente primero). Sin la faceta, se ignora y se usa el orden por defecto de siempre.

### 3.4 Carpetas de `Collections/`

- Un segmento de ruta llamado `Collections` (sin distinguir mayúsculas) marca una carpeta de colecciones; cada subcarpeta directa es una candidata. Si hay un `Collections` dentro de otro, cuenta el primero.
- Su contenido: todas las versiones presentes debajo de la subcarpeta, a cualquier profundidad, salvo las marcadas `extra` o `ignored`. Las versiones sin huella se omiten (no se pueden guardar como entrada).
- Pendiente = sin decisión y con contenido, o `imported` con novedades (§2). Una candidata sin versiones presentes no se muestra.
- Para cada pendiente: ruta, nombre (el de la subcarpeta), cantidad total, cantidad de novedades, hasta 8 afiches y, si ya se importó, la lista de destino.

### 3.5 Inicio

Tipo de fila nuevo `RowList = "list"`: hasta 2 listas, las de `updated_at` más reciente primero, con al menos 3 películas identificadas presentes cada una (las filas de Inicio solo muestran películas), de la agregada más recientemente a la más antigua. "Ver todas →" lleva a `/explorar?lista=<id>`. Entran al orden aleatorio de las filas como las demás: Inicio ya mezcla todas sus filas, "agregadas recientemente" incluida.

## 4. API

Las escrituras pasan por `jsonOnly`, como las existentes.

| Ruta | Qué hace |
|---|---|
| `GET /api/lists` | Índice: `id`, `name`, `count` (ítems presentes), `updatedAt`, `cover` (imagen de escena del ítem agregado más recientemente que tenga una). Orden: `updatedAt` descendente. |
| `POST /api/lists` `{name}` | Crea; responde 201 con `{id, name}`. |
| `PATCH /api/lists/{id}` `{name}` | Renombra. |
| `DELETE /api/lists/{id}` | Borra la lista (nunca archivos). |
| `POST /api/lists/{id}/entries` `{tmdbId}` o `{key}` | Agrega una película o un ítem sin identificar. Idempotente. |
| `DELETE /api/lists/{id}/entries` `{tmdbId}` o `{key}` | Quita las entradas que llevan a ese ítem (§2). |
| `GET /api/collections` | Carpetas pendientes (§3.4). |
| `POST /api/collections/import` `{path, name}` o `{path, listId}` | Importa como lista nueva, a una lista existente, o suma novedades a la lista de una importación previa. Agrega solo las novedades y las marca como vistas. Responde `{listId}`. |
| `POST /api/collections/dismiss` `{path}` | Descarta: marca todo el contenido actual como visto. |

`GET /api/movies/{id}` y `GET /api/versions/{key}` suman `lists: [{id, name}]`: las listas que contienen ese ítem.

`GET /api/explore` acepta `lista=` y `orden=agregado-lista`.

Errores:

- Nombre vacío o solo espacios, o de más de 100 caracteres → 400. Los espacios de los extremos se recortan.
- Nombre repetido (sin distinguir mayúsculas) → 409.
- Lista, película, ítem o carpeta desconocida → 404. Una carpeta es desconocida si no es una candidata pendiente.
- `{name}` y `{listId}` juntos, o ninguno → 400.

## 5. Interfaz

Svelte, textos en español (es-AR), siguiendo las páginas y componentes existentes.

- **Nav:** Inicio · Explorar · **Listas** · Revisar.
- **Listas (`/listas`, página nueva `Listas.svelte`):** grilla de tarjetas con imagen de escena, nombre en mayúsculas y "N películas". Botón "Nueva lista" con un campo de nombre en el lugar. Cada tarjeta tiene un menú ⋯ con Renombrar y Borrar; Borrar pide confirmación y aclara que no se tocan archivos. Clic en la tarjeta → `/explorar?lista=<id>`. Sin listas, un texto que explica cómo crearlas e invita a mirar Revisar → Colecciones.
- **Explorar con una lista:** el título de la página es el nombre de la lista. La faceta `Lista` está en `BarraFacetas`. El orden "Agregado a la lista" aparece solo con una lista aplicada. Con una lista aplicada, cada afiche tiene la acción "Quitar de la lista".
- **Ficha de película y ficha de versión:** chips con las listas que contienen el ítem (clic → la lista) y un chip "+". El "+" abre un menú con todas las listas marcadas o no (marcar agrega, desmarcar quita), un filtro por nombre y un campo para crear una lista nueva que ya contiene el ítem.
- **Revisar → Colecciones (`/revisar/colecciones`, tercera pestaña):** una fila por carpeta pendiente con nombre, ruta, "N películas" (o "N nuevas" si ya se importó) y una tira de afiches. Acciones:
  - **Importar como lista**, con el nombre de la carpeta prellenado y editable. Si ya existe una lista con ese nombre, la acción por defecto pasa a ser "Agregar a *esa lista*".
  - **Agregar a…** una lista existente.
  - **Descartar.**
  - En una carpeta ya importada: **Agregar N nuevas a *lista*** y **Descartar novedades**.
- **Inicio:** las filas de listas usan `FilaInicio`.
- **Lógica pura** (validación de nombres, referencias de entradas, qué acción corresponde en Colecciones) en `web/src/lib/listas.js`, con `listas.test.js`, como `partes.js`.

## 6. Pruebas

- **store:** crear, renombrar y borrar listas (cascada de entradas, `list_id` nulo en carpetas); nombre repetido sin distinguir mayúsculas; entradas idempotentes; importar y descartar registran lo visto; `Snapshot` trae listas y carpetas.
- **catalog:** resolución de entradas (película, huella identificada, huella que pasó a `extra`, entradas duplicadas, contenido ausente); detección de carpetas (mayúsculas, profundidad, `Collections` anidado, extras omitidos, novedades después de importar, descartadas que no vuelven); faceta `lista` y orden `agregado-lista`; fila de Inicio.
- **server:** cada ruta con sus casos 400, 404 y 409, y la guarda de `jsonOnly`.
- **web:** `listas.test.js`; ajustes en `facets.test.js` y `home.test.js`.
- **Prueba real (última tarea del plan):** importar desde `D:\cine\Collections`, reescanear y comprobar que las listas, las entradas y las decisiones sobreviven; corregir la identificación de un archivo importado y ver que la lista muestra la película corregida.
