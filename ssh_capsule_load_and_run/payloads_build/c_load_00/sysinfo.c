#include <stdio.h>
#include <stdlib.h>
#include <unistd.h>

int main() {
    printf("=== Ubuntu Machine Info ===\n");

    // Hostname
    char hostname[256];
    if (gethostname(hostname, sizeof(hostname)) == 0) {
        printf("Hostname: %s\n", hostname);
    }

    // Current user
    char *user = getenv("USER");
    if (user != NULL) {
        printf("User: %s\n", user);
    }

    // Uname info
    char buf[256];
    if (system("uname -a") == 0) {
        printf("Full uname info above.\n");
    }

    // Memory info
    if (system("cat /proc/meminfo | head -n 3") == 0) {
        printf("Memory info above.\n");
    }

    return 0;
}
