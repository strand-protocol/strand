/*
 * test_main.c - NexRoute test runner
 *
 * Minimal test framework: each test file registers test functions.
 * We run them all and report pass/fail counts.
 */

#include <stdio.h>
#include <stdlib.h>
#include <string.h>

/* --------------------------------------------------------------------------
 * Test framework
 * -------------------------------------------------------------------------- */

typedef int (*test_fn_t)(void);

typedef struct {
    const char *name;
    test_fn_t   fn;
} test_case_t;

#define MAX_TESTS 256

static test_case_t g_tests[MAX_TESTS];
static int         g_num_tests = 0;

void test_register(const char *name, test_fn_t fn)
{
    if (g_num_tests < MAX_TESTS) {
        g_tests[g_num_tests].name = name;
        g_tests[g_num_tests].fn   = fn;
        g_num_tests++;
    }
}

/* --------------------------------------------------------------------------
 * Assertion helper (for use in test files)
 * -------------------------------------------------------------------------- */

int test_assert_impl(int cond, const char *expr, const char *file, int line)
{
    if (!cond) {
        fprintf(stderr, "  FAIL: %s (%s:%d)\n", expr, file, line);
        return 1;
    }
    return 0;
}

/* --------------------------------------------------------------------------
 * Registration functions from test files
 * -------------------------------------------------------------------------- */

extern void register_sad_tests(void);
extern void register_routing_tests(void);

/* --------------------------------------------------------------------------
 * Main
 * -------------------------------------------------------------------------- */

int main(int argc, char **argv)
{
    (void)argc;
    (void)argv;

    /* Register all test suites */
    register_sad_tests();
    register_routing_tests();

    printf("NexRoute Test Suite: %d tests\n", g_num_tests);
    printf("========================================\n");

    int passed = 0;
    int failed = 0;

    for (int i = 0; i < g_num_tests; i++) {
        printf("[%3d/%3d] %-50s ", i + 1, g_num_tests, g_tests[i].name);
        fflush(stdout);

        int rc = g_tests[i].fn();
        if (rc == 0) {
            printf("PASS\n");
            passed++;
        } else {
            printf("FAIL (%d errors)\n", rc);
            failed++;
        }
    }

    printf("========================================\n");
    printf("Results: %d passed, %d failed, %d total\n",
           passed, failed, passed + failed);

    return (failed > 0) ? 1 : 0;
}
