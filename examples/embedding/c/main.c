#include <stdio.h>
#include "libgocdm.h"

int main(void) {
  char* out = GoCDMDefaultConfigJSON();
  if (out) {
    puts(out);
    GoCDMFreeCString(out);
  }
  return 0;
}
