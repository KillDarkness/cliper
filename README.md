# Cliper

Cliper é um aplicativo de clipping para Linux escrito em Go que mantém um buffer contínuo da tela e salva um MP4 final quando uma hotkey global é pressionada.

## Arquitetura

O projeto é dividido em quatro módulos principais:

- **Config (`internal/config`)**: centraliza caminhos, duração dos segmentos, quantidade máxima de segmentos, FPS, resolução, hotkey, backend de captura e parâmetros de encoder. Também cria automaticamente `buffer/` e `clips/` na inicialização.
- **FFmpeg manager (`internal/ffmpeg`)**: inicia o FFmpeg via `exec.Command`, mantém a gravação contínua em background, grava segmentos MPEG-TS pequenos e reinicia o processo se ele encerrar inesperadamente.
- **Clip saver (`internal/clip`)**: lê a playlist de segmentos finalizados, copia um snapshot dos segmentos atuais para um diretório temporário e concatena tudo com FFmpeg usando `-c copy`, evitando reencodificação na hora de salvar o clip.
- **Hotkey manager (`internal/hotkey`)**: registra uma hotkey global no X11 usando XGrabKey. No Wayland, hotkeys globais são restritas por design; nesse caso o app mantém um fallback por terminal, pressionando Enter para salvar um clip.

## Fluxo de funcionamento

1. Na inicialização, `buffer/` e `clips/` são criados automaticamente.
2. No X11, se `CLIPER_VIDEO_SIZE` não for definido, o app tenta detectar automaticamente a resolução atual do monitor usando `xdpyinfo` e depois `xrandr`.
3. O FFmpeg começa a capturar a tela e gera segmentos `.ts` em `buffer/`.
4. A playlist `buffer/segments.ffconcat` mantém somente os segmentos finalizados mais recentes.
5. Ao pressionar a hotkey, o clip saver copia os segmentos listados para um snapshot temporário.
6. O FFmpeg concatena o snapshot com `-c copy` e salva `clips/clip_TIMESTAMP.mp4`.

A cópia para snapshot evita que segmentos sejam removidos ou alterados durante a concatenação.

## Requisitos no Arch Linux

```bash
sudo pacman -S ffmpeg go libx11 xorg-xdpyinfo xorg-xrandr
```

Para X11, o backend padrão usa `x11grab`. Em Wayland, use `pipewire` se o seu FFmpeg tiver suporte ao input PipeWire, ou `kmsgrab` em cenários avançados com permissões adequadas.

## Uso

```bash
go run ./cmd/cliper
```

Por padrão, a hotkey é `F8` no X11. A configuração padrão é:

- resolução: automática no X11 (`xdpyinfo`/`xrandr`), com fallback para `1920x1080` se a detecção falhar;
- FPS: `60`;
- segmento: `5` segundos;
- janela de buffer: `24` segmentos, ou seja, aproximadamente `2` minutos;
- encoder: `libx264`, preset `veryfast`, CRF `23`, pixel format `yuv420p`.

## Variáveis de ambiente

### Captura

- `CLIPER_BACKEND`: `x11`, `pipewire` ou `kmsgrab`.
- `CLIPER_DISPLAY`: display X11, padrão `$DISPLAY` ou `:0.0`.
- `CLIPER_VIDEO_SIZE`: resolução `WIDTHxHEIGHT` ou `auto`. Se não for informado, usa `auto`.
- `CLIPER_CAPTURE_OFFSET`: offset X11 no formato `X,Y`, padrão `0,0`. Exemplo: `1366,0` para capturar o segundo monitor à direita.
- `CLIPER_PIPEWIRE_NODE`: node/device usado no backend PipeWire, padrão `0`.
- `CLIPER_DRAW_MOUSE`: `true`/`false`, padrão `true`.
- `CLIPER_FPS`: FPS, padrão `60`.

### Buffer e clip

- `CLIPER_SEGMENT_SECONDS`: duração de cada segmento, padrão `5`.
- `CLIPER_MAX_SEGMENTS`: quantidade de segmentos mantidos na playlist, padrão `24`.
- `CLIPER_BUFFER_DIR`: diretório de buffer, padrão `buffer`.
- `CLIPER_CLIPS_DIR`: diretório dos clips, padrão `clips`.

### Encoder e FFmpeg

- `CLIPER_VIDEO_CODEC`: codec de vídeo usado pelo FFmpeg, padrão `libx264`.
- `CLIPER_PRESET`: preset do encoder, padrão `veryfast`.
- `CLIPER_CRF`: CRF do encoder, padrão `23`.
- `CLIPER_PIXEL_FORMAT`: pixel format, padrão `yuv420p`.
- `CLIPER_FFMPEG`: caminho do binário FFmpeg, padrão `ffmpeg`.
- `CLIPER_RESTART_DELAY_SECONDS`: atraso antes de reiniciar o FFmpeg, padrão `2`.
- `CLIPER_CLEANUP_INTERVAL_SECONDS`: intervalo de limpeza de segmentos antigos, padrão `10`.

### Hotkey

- `CLIPER_HOTKEY`: tecla global no X11, padrão `F8`.

## Exemplos

Capturar automaticamente a resolução atual do X11:

```bash
go run ./cmd/cliper
```

Capturar manualmente uma tela 1366x768:

```bash
CLIPER_VIDEO_SIZE=1366x768 go run ./cmd/cliper
```

Capturar uma região/monitor com offset em X11:

```bash
CLIPER_VIDEO_SIZE=1920x1080 CLIPER_CAPTURE_OFFSET=1366,0 go run ./cmd/cliper
```

Alterar duração do buffer para 3 minutos com segmentos de 5 segundos:

```bash
CLIPER_SEGMENT_SECONDS=5 CLIPER_MAX_SEGMENTS=36 go run ./cmd/cliper
```

Reduzir uso de CPU diminuindo FPS e aumentando CRF:

```bash
CLIPER_FPS=30 CLIPER_CRF=28 CLIPER_PRESET=ultrafast go run ./cmd/cliper
```

Usar PipeWire experimental:

```bash
CLIPER_BACKEND=pipewire CLIPER_PIPEWIRE_NODE=0 CLIPER_VIDEO_SIZE=1920x1080 go run ./cmd/cliper
```

## Observações sobre Wayland

Wayland não permite hotkeys globais arbitrárias para clientes comuns. Para uso completo em Wayland, a abordagem recomendada é integrar com atalhos do compositor chamando uma interface externa do app. Esta versão mantém suporte experimental a captura PipeWire/KMSGrab, mas a hotkey global nativa é implementada para X11.
