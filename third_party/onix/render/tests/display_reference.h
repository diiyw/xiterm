#ifndef ONIX_DISPLAY_REFERENCE_H
#define ONIX_DISPLAY_REFERENCE_H
// CPU reference for ACESFilmicToneMapping's published sRGB/AP1 transforms
// and Stephen Hill RRT/ODT fit. Row-major matrices intentionally differ from
// the shader's column-major constructor representation.
static void aces_display_reference(const float rgb[3], unsigned char result[3]) {
 const double input[3][3]={{.59719,.35458,.04823},{.076,.90834,.01566},{.0284,.13383,.83777}};
 const double output[3][3]={{1.60475,-.53108,-.07367},{-.10208,1.10813,-.00605},{-.00327,-.07276,1.07602}};
 double fit[3];
 for(int row=0;row<3;row++) {
  double v=0;for(int c=0;c<3;c++) v+=input[row][c]*rgb[c]/.6;
  fit[row]=(v*(v+.0245786)-.000090537)/(v*(.983729*v+.432951)+.238081);
 }
 for(int row=0;row<3;row++) {
  double v=0;for(int c=0;c<3;c++) v+=output[row][c]*fit[c];
  v=fmax(0,fmin(1,v));
  result[row]=(unsigned char)lround((v<=.0031308?12.92*v:1.055*pow(v,1/2.4)-.055)*255);
 }
}
#endif
