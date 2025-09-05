#include <stdio.h>
#include <stdlib.h>
#include <string.h>  

int main(int argc, char *argv[]) {
    printf("=== MyTool Info ===\n");

    if (argc > 1 && (strcmp(argv[1], "--version") == 0 || strcmp(argv[1], "-v") == 0)) {
        printf("mytool version 1.0.0\n");
        return 0;
    }

    printf("Hello from mytool!\n");
    printf("You passed %d arguments.\n", argc - 1);
    for (int i = 1; i < argc; i++) {
        printf("arg[%d]: %s\n", i, argv[i]);
    }

    return 0;
}
