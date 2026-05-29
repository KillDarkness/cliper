# Cliper

Cliper é um aplicativo de clipping para Linux escrito em Go que mantém um buffer contínuo dos últimos 2 minutos da tela e salva um MP4 final quando uma hotkey global é pressionada.

## Arquitetura

O projeto é dividido em quatro módulos principais:

- **Config (`internal/config`)**: centraliza caminhos, duração dos segmentos, quantidade máxima de segmentos, FPS, resolução, offset da captura, parâmetros de encoder, hotkey e backend de captura. Também cria automaticamente `buffer/` e `clips/` na inicialização.
- **FFmpeg manager (`internal/ffmpeg`)**: inicia o FFmpeg via `exec.Command`, mantém a gravação contínua em background, grava segmentos MPEG-TS de 5 segundos e reinicia o processo se ele encerrar inesperadamente.
- **Clip saver (`internal/clip`)**: lê a playlist de segmentos finalizados, copia um snapshot dos segmentos atuais para um diretório temporário e concatena tudo com FFmpeg usando `-c copy`, evitando reencodificação e reduzindo uso de CPU.
- **Hotkey manager (`internal/hotkey`)**: mantém um fallback por terminal, pressionando Enter para salvar um clip. Opcionalmente, builds Linux com CGO e a tag `cliper_x11` registram uma hotkey global no X11 usando XGrabKey. No Wayland, hotkeys globais são restritas por design; use o fallback por terminal ou atalhos do compositor.

## Fluxo de funcionamento

1. Na inicialização, `buffer/` e `clips/` são criados automaticamente.
2. O FFmpeg começa a capturar a tela e gera segmentos `.ts` de 5 segundos em `buffer/`.
3. A playlist `buffer/segments.ffconcat` mantém somente os últimos 24 segmentos finalizados, totalizando cerca de 2 minutos.
4. Ao pressionar a hotkey, o clip saver copia os segmentos listados para um snapshot temporário.
5. O FFmpeg concatena o snapshot com `-c copy` e salva `clips/clip_TIMESTAMP.mp4`.

A cópia para snapshot evita que segmentos sejam removidos ou alterados durante a concatenação.

## Requisitos no Arch Linux

```bash
sudo pacman -S ffmpeg go libx11
# Opcional, mas recomendado para detecção automática de resolução no X11:
sudo pacman -S xorg-xdpyinfo xorg-xrandr
```

Para X11, o backend padrão usa `x11grab`. Em Wayland, use `pipewire` se o seu FFmpeg tiver suporte ao input PipeWire, ou `kmsgrab` em cenários avançados com permissões adequadas.

## Uso

```bash
go run ./cmd/cliper
```

Por padrão, `go run ./cmd/cliper` usa o fallback por terminal (pressione Enter para salvar) para evitar depender de CGO/libX11 durante a compilação. Para habilitar a hotkey global `F8` no X11, instale `libx11` e execute com a tag opcional:

```bash
go run -tags cliper_x11 ./cmd/cliper
```

Variáveis de ambiente úteis:

- `CLIPER_BACKEND`: `x11`, `pipewire` ou `kmsgrab`.
- `CLIPER_DISPLAY`: display X11, padrão `$DISPLAY` ou `:0.0`.
- `CLIPER_VIDEO_SIZE`: resolução da captura. O padrão é `auto`; no X11 o app tenta detectar a resolução atual com `xdpyinfo`/`xrandr` e só usa `1920x1080` como fallback se não conseguir detectar. Também aceita valores manuais como `1366x768`.
- `CLIPER_CAPTURE_OFFSET`: posição inicial da captura no X11 em formato `+X,Y`, padrão `+0,0`. Exemplo: `+1920,0` para capturar um segundo monitor à direita.
- `CLIPER_FPS`: FPS, padrão `60`.
- `CLIPER_VIDEO_CODEC`: codec de vídeo passado para `-c:v`, padrão `libx264`.
- `CLIPER_VIDEO_PRESET`: preset do encoder, padrão `veryfast`; deixe vazio para omitir `-preset`.
- `CLIPER_VIDEO_TUNE`: tune do encoder, padrão `zerolatency`; deixe vazio para omitir `-tune`.
- `CLIPER_VIDEO_CRF`: CRF do encoder; por padrão fica vazio e o parâmetro é omitido.
- `CLIPER_PIXEL_FORMAT`: formato de pixel, padrão `yuv420p`; deixe vazio para omitir `-pix_fmt`.
- `CLIPER_SEGMENT_DURATION`: duração de cada segmento, padrão `5s`. Também aceita número inteiro em segundos.
- `CLIPER_MAX_SEGMENTS`: quantidade máxima de segmentos na playlist, padrão `24`.
- `CLIPER_RESTART_DELAY`: espera antes de reiniciar o FFmpeg se ele cair, padrão `2s`.
- `CLIPER_CLEANUP_INTERVAL`: intervalo de limpeza de segmentos antigos, padrão `10s`.
- `CLIPER_HOTKEY`: `F8`, `F9`, `F10`, `F11` ou `F12`.
- `CLIPER_FFMPEG`: caminho do binário FFmpeg, padrão `ffmpeg`.
- `CLIPER_BUFFER_DIR`: diretório de buffer, padrão `buffer`.
- `CLIPER_CLIPS_DIR`: diretório dos clips, padrão `clips`.

Exemplos:

```bash
CLIPER_BACKEND=x11 CLIPER_DISPLAY=:0.0 go run ./cmd/cliper
CLIPER_BACKEND=x11 CLIPER_DISPLAY=:0.0 go run -tags cliper_x11 ./cmd/cliper
CLIPER_BACKEND=x11 CLIPER_DISPLAY=:0.0 CLIPER_VIDEO_SIZE=1366x768 go run ./cmd/cliper
CLIPER_BACKEND=x11 CLIPER_CAPTURE_OFFSET=+1920,0 go run ./cmd/cliper
CLIPER_BACKEND=pipewire CLIPER_VIDEO_SIZE=1366x768 go run ./cmd/cliper
CLIPER_BACKEND=kmsgrab CLIPER_VIDEO_SIZE=1366x768 go run ./cmd/cliper
```


### Salvar um clip manualmente depois de parar o Cliper

Se você matou/parou o `cliper`, os segmentos finalizados continuam em `buffer/` junto com a playlist `buffer/segments.ffconcat`. Você pode juntar esses segmentos depois, sem iniciar uma nova gravação, usando o subcomando `save` (aliases: `clip` e `join`):

```bash
go run ./cmd/cliper save
```

Por padrão, esse comando junta todos os segmentos finalizados que ainda estão na playlist. Para salvar só o final do buffer, defina um tempo com `-duration`/`-d`:

```bash
go run ./cmd/cliper save -duration 30s
go run ./cmd/cliper save -d 1m30s -output clips/ultimos-90s.mp4
```

Opções do subcomando:

- `-duration`/`-d`: duração desejada a partir do fim do buffer. `0` salva todos os segmentos finalizados na playlist.
- `-output`/`-o`: caminho do MP4 de saída. Se omitido, salva em `CLIPER_CLIPS_DIR/clip_TIMESTAMP.mp4`.
- `-timeout`: tempo máximo para esperar o FFmpeg concatenar o clip, padrão `10m`.

## Observações sobre Wayland

Wayland não permite hotkeys globais arbitrárias para clientes comuns. Para uso completo em Wayland, a abordagem recomendada é integrar com atalhos do compositor chamando uma interface externa do app. Esta versão mantém suporte experimental a captura PipeWire/KMSGrab. A hotkey global nativa é implementada apenas para builds X11 explícitos com `-tags cliper_x11`; builds padrão continuam usando o fallback por terminal.
