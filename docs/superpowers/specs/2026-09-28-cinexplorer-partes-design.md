# Cinexplorer — Películas en partes — Diseño

Fecha: 2026-09-28
Estado: aprobado en la conversación; pendiente de revisión del texto
Spec general: `2026-09-25-cinexplorer-design.md`
Relacionado: `2026-09-26-cinexplorer-m3-identificacion-design.md` (correcciones manuales)

## 1. Objetivo

Que una película guardada en varios archivos (p. ej. *Shoah*, 7 partes) sea **una sola versión** en el catálogo, con dos caminos:

- **Automático:** archivos de la misma carpeta con marcador de parte (`Part 1`, `CD2`, `Parte 3`…) aunque lo que sigue al marcador difiera (`Shoah - Part 1 - Auschwitz.mkv`).
- **Manual:** un comando en la interfaz para decir "esta versión es una parte de aquella", para lo que ninguna heurística resuelve.

## 2. Situación actual

- `grouping.Build` ya une en una versión los archivos de una carpeta cuyo nombre solo difiere en el marcador (`splitPart`). El resto del nombre debe ser idéntico.
- La identidad de una versión es la huella de su archivo representante (`representatives`: el de menor `part`). Si las partes no se unen, cada una es una versión con su propia identificación, aparece en Revisar y, si todas resuelven a la misma película, se marcan como duplicadas.
- Las versiones se reconstruyen en cada escaneo (`ReplaceVersionsIn`), así que un agrupado manual no puede vivir en la fila de la versión: es una corrección del usuario y se guarda aparte, por huella, como `extra` o `ignored`.

## 3. Agrupado automático

En `buildDir`, cuando un video tiene marcador de parte, la clave de agrupado pasa a ser **el texto anterior al marcador** (en minúsculas, sin separadores finales), no el nombre entero sin el marcador.

Salvaguardas:

1. Un grupo por prefijo solo se forma con **al menos 2 archivos y números de parte distintos**. Un `Película - Part 1 - Subtítulo.mkv` suelto conserva el comportamiento actual.
2. Los archivos de un grupo así quedan **exentos de la regla de extra por tamaño** (menos del 15 % del video más grande). Los marcadores `bonus`, `trailer`, etc. y las carpetas de extras siguen valiendo.
3. Si el marcador es lo último del nombre (`Novecento - Part 1`), el resultado es el mismo que hoy.
4. Los subtítulos siguen asociándose por prefijo del nombre base; con el prefijo nuevo como clave, `Shoah - Part 2 - Treblinka.es.srt` cae en la versión correcta.
5. El nombre parseado (`Parsed`) sale del prefijo, no de un nombre con el título de cada parte pegado.
6. Si un número de parte se repite dentro del mismo prefijo (`Movie.CD1.720p`, `Movie.CD2.720p`, `Movie.CD1.1080p`, `Movie.CD2.1080p`), son varias copias de la misma película y no se agrupan por prefijo: queda el comportamiento actual.
7. Se agrupa por prefijo solo si los nombres base difieren; si son iguales salvo el marcador, sigue valiendo el agrupado de siempre (que conserva las etiquetas de calidad que siguen al marcador, como `720p`). La exención de la regla de extra por tamaño vale para todo conjunto de partes, también para los de mismo nombre.

## 4. Agrupado manual

### 4.1 Datos

Tabla nueva en `schema.sql`:

```sql
CREATE TABLE IF NOT EXISTS part_links (
  fingerprint TEXT PRIMARY KEY,  -- huella del representante de la versión que se une
  leader      TEXT NOT NULL,     -- huella del representante de la versión que la recibe
  created_at  INTEGER NOT NULL
);
```

Va por huella de contenido: sobrevive a renombres, movidas y reescaneos, igual que `identifications`.

### 4.2 Aplicación

Un paso del store, `ApplyPartLinks`, corre **después** de `ReplaceVersionsIn` y sobre lo ya persistido (así funciona también cuando un escaneo parcial solo tocó una de las raíces):

- Por cada vínculo cuyo seguidor y líder existen como versiones distintas: los archivos del seguidor pasan a la versión del líder, con `part` numerado a continuación de las partes del líder y ordenado por ruta; se recalculan `size` y `parts`, y se borra la versión vacía.
- Si falta el líder (no encontrado o vinculado él mismo), el vínculo no se aplica y se conserva.
- Un líder que a su vez es seguidor se resuelve siguiendo la cadena; los ciclos se ignoran.
- Es idempotente: si ya está aplicado, no hace nada.

Al pasar a ser parte de otra, la identificación del seguidor deja de contar (ya no es representante de ninguna versión); manda la del líder.

### 4.3 API y estado

- Acciones nuevas en el endpoint de identificación existente (`internal/server/identify.go`): `part-of` (con la huella del líder) y `unlink`.
- Modo consulta (solo lectura): sin acciones, como el resto de las correcciones.
- `unlink` borra el vínculo y el servidor lanza un reescaneo de las raíces afectadas, que reconstruye las versiones separadas (`ApplyPartLinks` ya no las vuelve a unir).

### 4.4 Interfaz

En `TarjetaVersion.svelte`, menú ⋯:

- **"Es una parte de…"**: abre un selector como el de "Es un extra de…" (`SelectorExtra.svelte`), que busca una película del catálogo y elige una de sus versiones como líder.
- **"Separar de la película"**: solo en versiones con partes vinculadas a mano.

Sin cambios en ▶ Ver: sigue abriendo la primera parte.

## 5. Pruebas

- `grouping`: nombres tipo Shoah se unen; un parte-con-subtítulo suelto no se une; una parte corta no se toma como extra; subtítulos van a la versión correcta; los casos existentes no cambian.
- `store`: vínculo aplicado tras un reescaneo; sobrevive a mover el archivo; cadena y ciclo; líder ausente conserva el vínculo; `unlink`.
- `server`: acciones `part-of` y `unlink`, y rechazo en modo consulta.
- `web`: entradas del menú y llamada a la API.

## 6. Fuera de alcance

- Números romanos, "1 of 7", `E01` y una carpeta por parte.
- Reordenar partes a mano (el orden es por ruta).
- Cambiar cómo ▶ Ver reproduce películas en varias partes.
