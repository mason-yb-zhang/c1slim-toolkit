// c1dec: MP3 stream decoder bridge for C1-Slim.
// stdin  = MP3 byte stream (curl pipe)
// Decodes via minimp3 and pipes raw S16_LE PCM into aplay (spawned after the
// first frame so sample rate and channel count are known).
#include <stdio.h>
#include <stdlib.h>
#include <string.h>
#include <unistd.h>
#include <sys/types.h>
#include <sys/wait.h>

#define MINIMP3_IMPLEMENTATION
#include "minimp3.h"

#define READ_CHUNK 16384
#define BUF_CAP (READ_CHUNK * 4)

static mp3dec_t mp3d;
static pid_t aplay_pid = -1;
static FILE *aplay_in;

static void ensure_aplay(int rate, int channels)
{
    if (aplay_in)
        return;
    int fds[2] = {-1, -1};
    pipe(fds);
    if (fds[0] < 0) {
        fprintf(stderr, "pipe failed\n");
        exit(1);
    }
    aplay_pid = fork();
    if (aplay_pid == 0) {
        dup2(fds[0], STDIN_FILENO);
        close(fds[0]);
        close(fds[1]);
        char r[16], c[16];
        snprintf(r, sizeof r, "%d", rate);
        snprintf(c, sizeof c, "%d", channels);
        execlp("/usr/bin/aplay", "aplay", "-q", "-D", "hw:0,0",
               "-B", "20000", "-R", "0", "-T", "500000",
               "-f", "S16_LE", "-r", r, "-c", c, "-t", "raw", (char *)NULL);
        perror("exec aplay");
        _exit(1);
    }
    close(fds[0]);
    aplay_in = fdopen(fds[1], "wb");
    fprintf(stderr, "c1dec: aplay rate=%d channels=%d\n", rate, channels);
}

int main(void)
{
    static unsigned char buf[BUF_CAP];
    size_t len = 0;
    mp3dec_init(&mp3d);

    for (;;) {
        if (len < READ_CHUNK) {
            ssize_t got = read(STDIN_FILENO, buf + len, BUF_CAP - len);
            if (got <= 0) {
                // EOF: drain what is left, then exit.
                break;
            }
            len += (size_t)got;
        }
        mp3dec_frame_info_t info;
        short pcm[MINIMP3_MAX_SAMPLES_PER_FRAME];
        int samples = mp3dec_decode_frame(&mp3d, buf, (int)len, pcm, &info);
        if (info.frame_bytes > 0) {
            if (samples > 0) {
                ensure_aplay(info.hz, info.channels);
                fwrite(pcm, sizeof(short) * (size_t)info.channels,
                       (size_t)samples, aplay_in);
            }
            memmove(buf, buf + info.frame_bytes, len - (size_t)info.frame_bytes);
            len -= (size_t)info.frame_bytes;
        } else {
            // No sync / not enough data: drop one byte and continue.
            memmove(buf, buf + 1, --len);
        }
    }

    if (aplay_in) {
        fclose(aplay_in);
        int status;
        waitpid(aplay_pid, &status, 0);
    }
    return 0;
}
