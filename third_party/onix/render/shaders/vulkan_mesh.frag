#version 450
// Generated from mesh.frag.wgsl.




struct PBRUniforms {
    mat4x4 projection;
    vec4 cameraPosition;
    vec4 ambientLight;
    vec4 lightDirection;
    vec4 directionalLight;
    vec4 lightPosition[16];
    vec4 lightDirections[16];
    vec4 lightColors[16];
    vec4 lightParams[16];
    vec4 lightMeta;
    vec4 environment;
    vec4 renderParams;
    vec4 cameraForward;
    vec4 lightRight[16];
    vec4 lightUp[16];
    mat4x4 shadowMatrices[16];
    vec4 shadowRects[16];
    vec4 shadowParams[16];
    vec4 lightShadows[16];
    mat4x4 occlusionInverseProjection;
};
struct PBRInput {
    vec4 position;
    bool front;
    vec3 worldPosition;
    vec3 normal;
    vec2 uv;
    vec4 baseColor;
    vec4 emissiveMetallic;
    vec4 material;
    vec4 custom0_;
    vec4 custom1_;
    vec4 custom2_;
    vec4 custom3_;
    vec4 tangent;
};
struct PBRSurface {
    vec3 n;
    vec3 coatNormal;
    vec3 tangent;
    vec3 bitangent;
    vec3 base;
    float metallic;
    float roughness;
    vec3 f0_;
    float f90_;
    float diffuseF0_;
    float diffuseF90_;
    float coat;
    float coatRoughness;
    float coatFresnel;
    vec3 sheen;
    float sheenRoughness;
    float sheenAlbedo;
    float sheenScale;
    float sheenG1_;
    float anisotropy;
};
const float PI = 3.1415927;
const bool SHADOW_FLIP_Y = true;
const vec2 SSAO_DISK[32] = vec2[32](vec2(0.70710677, 0.0), vec2(-0.90308875, 0.8273033), vec2(0.1382322, -1.5750847), vec2(1.1382849, 1.484691), vec2(-2.0888927, -0.36949572), vec2(1.9787816, -1.2587389), vec2(-0.6618637, 2.4621), vec2(-1.2622459, -2.4303777), vec2(2.7385685, 1.0001209), vec2(-2.8490243, 1.1760359), vec2(1.373418, -2.9349146), vec2(1.014921, 3.235728), vec2(-3.0589836, -1.7727435), vec2(3.588536, -0.7889296), vec2(-2.1900277, 3.115089), vec2(-0.5059471, -3.9043589), vec2(3.106019, 2.6177561), vec2(-4.179728, 0.17284487), vec2(3.0487907, -3.033954), vec2(-0.2039759, 4.411167), vec2(-2.900934, -3.4762886), vec2(4.5954, 0.61830455), vec2(-3.893673, 2.7091162), vec2(1.0639755, -4.7294774), vec2(2.4609199, 4.294633), vec2(-4.8108625, -1.5347965), vec2(4.673141, -2.1591096), vec2(-2.0245214, 4.8374906), vec2(-1.8068408, -5.0234776), vec2(4.8078094, 2.5268497), vec2(-5.340266, 1.407677), vec2(3.0354443, -4.7208133));
const vec2 SHEEN_ALBEDO[512] = vec2[512](vec2(0.11262661, 0.99998283), vec2(0.1126264, 0.99625176), vec2(0.11262491, 0.97685343), vec2(0.11262091, 0.9372344), vec2(0.11261325, 0.8769452), vec2(0.112601, 0.79833186), vec2(0.11258361, 0.7054811), vec2(0.11256109, 0.6035446), vec2(0.11253432, 0.49817657), vec2(0.11250527, 0.39502022), vec2(0.11247739, 0.29922232), vec2(0.11245597, 0.21498416), vec2(0.11244861, 0.14517842), vec2(0.11246568, 0.0910856), vec2(0.11252095, 0.05231911), vec2(0.11263238, 0.02700419), vec2(0.11282307, 0.01223199), vec2(0.11312251, 0.00471917), vec2(0.11356826, 0.00149311), vec2(0.11420853, 0.00036927), vec2(0.11510586, 6.717e-5), vec2(0.11634301, 8.31e-6), vec2(0.11803218, 6.3e-7), vec2(0.12033072, 3e-8), vec2(0.12346846, 0.0), vec2(0.1277996, 0.0), vec2(0.13390803, 0.0), vec2(0.14284797, 0.0), vec2(0.15678708, 0.0), vec2(0.18120474, 0.0), vec2(0.23680517, 0.0), vec2(1.0, 0.0), vec2(0.12321281, 0.99998397), vec2(0.12321255, 0.9965348), vec2(0.12321077, 0.9785335), vec2(0.12320598, 0.94159245), vec2(0.12319679, 0.8850832), vec2(0.12318205, 0.8109517), vec2(0.12316103, 0.7227662), vec2(0.12313363, 0.62511706), vec2(0.12310065, 0.52312243), vec2(0.12306413, 0.42198113), vec2(0.12302772, 0.32655016), vec2(0.12299708, 0.2409498), vec2(0.12298035, 0.16821486), vec2(0.12298873, 0.11003024), vec2(0.1230371, 0.06660341), vec2(0.12314496, 0.03673093), vec2(0.12333735, 0.01809474), vec2(0.12364623, 0.00776682), vec2(0.12411231, 0.00281477), vec2(0.12478782, 0.00082766), vec2(0.12574051, 0.00018772), vec2(0.1270599, 3.078e-5), vec2(0.12886728, 3.36e-6), vec2(0.13133262, 2.2e-7), vec2(0.13470387, 1e-8), vec2(0.13936277, 0.0), vec2(0.1459378, 0.0), vec2(0.15556191, 0.0), vec2(0.17055999, 0.0), vec2(0.19679108, 0.0), vec2(0.25626886, 0.0), vec2(1.0, 0.0), vec2(0.1549085, 0.9999861), vec2(0.15490809, 0.9971643), vec2(0.15490527, 0.9822991), vec2(0.15489767, 0.95144343), vec2(0.15488309, 0.9036555), vec2(0.15485953, 0.84007317), vec2(0.15482561, 0.76317894), vec2(0.15478072, 0.67635167), vec2(0.15472537, 0.58350855), vec2(0.15466167, 0.48878813), vec2(0.15459368, 0.396255), vec2(0.154528, 0.30962017), vec2(0.15447433, 0.23198175), vec2(0.15444613, 0.1655997), vec2(0.15446158, 0.11172797), vec2(0.1545445, 0.07053544), vec2(0.15472552, 0.04114821), vec2(0.15504375, 0.02183588), vec2(0.15554897, 0.01033465), vec2(0.156305, 0.00425513), vec2(0.15739441, 0.00147674), vec2(0.15892608, 0.00041487), vec2(0.16104724, 8.954e-5), vec2(0.1639634, 1.386e-5), vec2(0.16797367, 1.4e-6), vec2(0.17353678, 8e-8), vec2(0.18140517, 0.0), vec2(0.19292893, 0.0), vec2(0.21086152, 0.0), vec2(0.24207935, 0.0), vec2(0.31197822, 0.0), vec2(1.0, 0.0), vec2(0.20747519, 0.99998814), vec2(0.20747447, 0.9978059), vec2(0.20746942, 0.98618364), vec2(0.20745577, 0.96173924), vec2(0.20742944, 0.9233483), vec2(0.20738675, 0.87146175), vec2(0.2073247, 0.8075756), vec2(0.2072414, 0.7339147), vec2(0.20713648, 0.6531915), vec2(0.20701167, 0.56839854), vec2(0.20687133, 0.48261958), vec2(0.20672311, 0.39885283), vec2(0.20657879, 0.3198438), vec2(0.20645534, 0.24793053), vec2(0.2063758, 0.18490694), vec2(0.20637058, 0.1319154), vec2(0.20647883, 0.08938218), vec2(0.2067507, 0.05701215), vec2(0.20724984, 0.03385681), vec2(0.20805776, 0.01846169), vec2(0.2092796, 0.0090833), vec2(0.21105339, 0.00394327), vec2(0.21356496, 0.00146739), vec2(0.21707243, 0.00045055), vec2(0.2219494, 0.00010841), vec2(0.22876564, 1.902e-5), vec2(0.23844951, 2.19e-6), vec2(0.25265372, 1.4e-7), vec2(0.27471384, 0.0), vec2(0.3128328, 0.0), vec2(0.39643002, 0.0), vec2(1.0, 0.0), vec2(0.28035367, 0.99998987), vec2(0.28035235, 0.99831647), vec2(0.28034315, 0.9893127), vec2(0.28031817, 0.97014606), vec2(0.28026992, 0.93966585), vec2(0.28019136, 0.89790004), vec2(0.28007638, 0.845674), vec2(0.27992046, 0.7843852), vec2(0.2797211, 0.7158389), vec2(0.2794786, 0.6421099), vec2(0.27919674, 0.56542116), vec2(0.27888373, 0.4880331), vec2(0.27855363, 0.4121398), vec2(0.27822727, 0.33977124), vec2(0.27793348, 0.27270326), vec2(0.27771053, 0.21237727), vec2(0.27760813, 0.15983328), vec2(0.27768996, 0.11566171), vec2(0.27803695, 0.07998042), vec2(0.27875218, 0.05244444), vec2(0.27996808, 0.03229446), vec2(0.28185758, 0.01844664), vec2(0.2846518, 0.00961955), vec2(0.28866944, 0.00448472), vec2(0.29436845, 0.001817), vec2(0.30244088, 0.00061493), vec2(0.31400412, 0.00016406), vec2(0.3310244, 3.154e-5), vec2(0.3574113, 3.75e-6), vec2(0.40256628, 2e-7), vec2(0.4987753, 0.0), vec2(1.0, 0.0), vec2(0.37244076, 0.9999911), vec2(0.37243846, 0.9986886), vec2(0.37242216, 0.9916199), vec2(0.37237808, 0.97642136), vec2(0.3722927, 0.95200485), vec2(0.37215328, 0.9181778), vec2(0.37194836, 0.8753607), vec2(0.37166855, 0.8244225), vec2(0.3713072, 0.7665614), vec2(0.37086117, 0.70320916), vec2(0.3703321, 0.6359491), vec2(0.3697279, 0.56644267), vec2(0.36906412, 0.4963623), vec2(0.3683649, 0.42732906), vec2(0.3676647, 0.36085585), vec2(0.3670102, 0.29829586), vec2(0.36646247, 0.24079598), vec2(0.36609995, 0.18925722), vec2(0.36602205, 0.14430277), vec2(0.36635506, 0.10625681), vec2(0.36726007, 0.07513601), vec2(0.36894572, 0.05065759), vec2(0.37168834, 0.03226631), vec2(0.37586534, 0.01918274), vec2(0.3820131, 0.01047232), vec2(0.3909327, 0.00513165), vec2(0.40390137, 0.00218329), vec2(0.4231317, 0.00076588), vec2(0.45293444, 0.00020286), vec2(0.5033589, 3.413e-5), vec2(0.606763, 2.33e-6), vec2(1.0, 0.0), vec2(0.48182863, 0.999992), vec2(0.48182476, 0.9989536), vec2(0.4817975, 0.993279), vec2(0.48172367, 0.9809815), vec2(0.4815805, 0.9610695), vec2(0.4813463, 0.93325055), vec2(0.4810011, 0.897713), vec2(0.48052758, 0.85500205), vec2(0.4799119, 0.8059282), vec2(0.47914478, 0.75149685), vec2(0.47822365, 0.69284976), vec2(0.47715428, 0.63121223), vec2(0.47595182, 0.56784606), vec2(0.4746425, 0.5040076), vec2(0.47326568, 0.44091076), vec2(0.47187582, 0.3796936), vec2(0.47054508, 0.3213887), vec2(0.4693664, 0.2668973), vec2(0.46845734, 0.2169672), vec2(0.46796656, 0.17217447), vec2(0.46808195, 0.13290927), vec2(0.4690443, 0.09936666), vec2(0.47116888, 0.07154291), vec2(0.47488034, 0.04923893), vec2(0.480774, 0.03207195), vec2(0.48972487, 0.01949708), vec2(0.5031047, 0.01084005), vec2(0.5232406, 0.00534144), vec2(0.5545416, 0.00221162), vec2(0.60682905, 0.00069341), vec2(0.7086414, 0.00012619), vec2(1.0, 2.27e-6), vec2(0.6055815, 0.9999927), vec2(0.6055754, 0.9991423), vec2(0.6055323, 0.99447054), vec2(0.60541564, 0.98428595), vec2(0.60518926, 0.9676975), vec2(0.60481846, 0.9443778), vec2(0.6042708, 0.9143866), vec2(0.60351723, 0.8780734), vec2(0.60253286, 0.83600473), vec2(0.60129917, 0.7889096), vec2(0.5998066, 0.7376345), vec2(0.598056, 0.68310183), vec2(0.59605974, 0.6262754), vec2(0.59384435, 0.56812966), vec2(0.5914526, 0.5096232), vec2(0.5889454, 0.45167607), vec2(0.5864049, 0.3951498), vec2(0.5839373, 0.34083146), vec2(0.58167666, 0.28941974), vec2(0.5797913, 0.24151447), vec2(0.57849175, 0.19760755), vec2(0.5780443, 0.15807642), vec2(0.5787919, 0.12317906), vec2(0.5811888, 0.09305054), vec2(0.58585966, 0.06770125), vec2(0.59370387, 0.04701694), vec2(0.6060986, 0.03076122), vec2(0.62531185, 0.0185808), vec2(0.6554624, 0.01001491), vec2(0.7050817, 0.00450958), vec2(0.7955515, 0.00143859), vec2(1.0, 0.00013284), vec2(0.7396568, 0.9999933), vec2(0.7396476, 0.99927765), vec2(0.7395834, 0.99533135), vec2(0.73940927, 0.9866908), vec2(0.7390713, 0.97255707), vec2(0.7385173, 0.95259964), vec2(0.737698, 0.9268096), vec2(0.7365681, 0.8954185), vec2(0.7350875, 0.8588401), vec2(0.73322535, 0.81762564), vec2(0.73096156, 0.7724264), vec2(0.72828865, 0.72396), vec2(0.725214, 0.6729825), vec2(0.7217622, 0.6202648), vec2(0.7179773, 0.56657153), vec2(0.7139254, 0.5126439), vec2(0.70969695, 0.45918515), vec2(0.7054097, 0.40684986), vec2(0.7012118, 0.35623458), vec2(0.6972878, 0.30787247), vec2(0.69386554, 0.2622287), vec2(0.6912282, 0.21969828), vec2(0.6897334, 0.18060496), vec2(0.68984365, 0.14520095), vec2(0.6921793, 0.11366716), vec2(0.69760716, 0.08611344), vec2(0.7074086, 0.06257905), vec2(0.7235987, 0.04303231), vec2(0.749602, 0.02737022), vec2(0.7918188, 0.01541781), vec2(0.8638102, 0.00692822), vec2(1.0, 0.00158595), vec2(0.8790779, 0.9999938), vec2(0.87906504, 0.9993756), vec2(0.87897426, 0.99595714), vec2(0.8787283, 0.9884494), vec2(0.8782507, 0.97613156), vec2(0.8774674, 0.9586844), vec2(0.8763079, 0.93606335), vec2(0.8747062, 0.9084292), vec2(0.8726035, 0.87609965), vec2(0.86995274, 0.8395117), vec2(0.86672, 0.7991881), vec2(0.8628867, 0.7557095), vec2(0.85845244, 0.7096912), vec2(0.85343766, 0.6617626), vec2(0.8478863, 0.61254984), vec2(0.84186786, 0.56266165), vec2(0.83547956, 0.5126774), vec2(0.8288489, 0.46313846), vec2(0.8221358, 0.41454163), vec2(0.8155375, 0.36733532), vec2(0.8092936, 0.3219173), vec2(0.80369556, 0.27863464), vec2(0.79910207, 0.2377848), vec2(0.7959626, 0.19961782), vec2(0.794858, 0.16433926), vec2(0.79656494, 0.13211331), vec2(0.8021717, 0.10306643), vec2(0.81327826, 0.07729056), vec2(0.8323765, 0.0548466), vec2(0.8636125, 0.03576848), vec2(0.91447544, 0.02006925), vec2(1.0, 0.00775292), vec2(1.018405, 0.99999434), vec2(1.0183877, 0.9994469), vec2(1.0182658, 0.9964143), vec2(1.0179356, 0.98973966), vec2(1.0172945, 0.97876596), vec2(1.0162426, 0.96318954), vec2(1.0146843, 0.94294834), vec2(1.0125294, 0.9181602), vec2(1.0096972, 0.88908225), vec2(1.0061215, 0.85607696), vec2(1.0017514, 0.81958264), vec2(0.99655485, 0.7800889), vec2(0.99052155, 0.7381164), vec2(0.98366636, 0.69419855), vec2(0.9760313, 0.64886653), vec2(0.96768814, 0.60263634), vec2(0.9587395, 0.5559983), vec2(0.9493214, 0.50940984), vec2(0.93960315, 0.4632888), vec2(0.92979145, 0.418011), vec2(0.9201326, 0.373908), vec2(0.9109182, 0.33126736), vec2(0.9024952, 0.29033402), vec2(0.8952801, 0.25131276), vec2(0.8897846, 0.21437138), vec2(0.88665193, 0.17964435), vec2(0.8867219, 0.14723697), vec2(0.8911312, 0.11722963), vec2(0.9014862, 0.08968239), vec2(0.9201696, 0.06464077), vec2(0.9509226, 0.04214378), vec2(1.0, 0.02223683), vec2(1.1524221, 0.99999493), vec2(1.1523998, 0.9994993), vec2(1.1522439, 0.9967497), vec2(1.151821, 0.9906889), vec2(1.151, 0.9807102), vec2(1.1496525, 0.9665254), vec2(1.1476551, 0.9480641), vec2(1.144891, 0.92541814), vec2(1.1412559, 0.89880544), vec2(1.1366615, 0.8685394), vec2(1.1310387, 0.83500224), vec2(1.1243398, 0.7986228), vec2(1.1165435, 0.75985825), vec2(1.1076578, 0.7191778), vec2(1.0977219, 0.6770485), vec2(1.0868087, 0.63392365), vec2(1.0750254, 0.59023273), vec2(1.0625149, 0.54637486), vec2(1.0494545, 0.502712), vec2(1.036057, 0.45956665), vec2(1.0225708, 0.41721928), vec2(1.009281, 0.375908), vec2(0.9965133, 0.3358296), vec2(0.98463917, 0.29714105), vec2(0.97408736, 0.25996214), vec2(0.96535975, 0.22437812), vec2(0.959059, 0.19044288), vec2(0.95592993, 0.15818204), vec2(0.9569234, 0.127596), vec2(0.9633017, 0.09866326), vec2(0.976818, 0.0713445), vec2(1.0, 0.04558729), vec2(1.2768352, 0.9999958), vec2(1.2768079, 0.9995382), vec2(1.2766165, 0.99699694), vec2(1.2760974, 0.9913896), vec2(1.2750894, 0.98214805), vec2(1.2734348, 0.9689979), vec2(1.2709811, 0.9518648), vec2(1.2675841, 0.9308244), vec2(1.2631145, 0.90606886), vec2(1.2574618, 0.87787884), vec2(1.2505368, 0.84659845), vec2(1.2422762, 0.8126157), vec2(1.2326468, 0.77634513), vec2(1.2216488, 0.7382126), vec2(1.2093182, 0.69864243), vec2(1.1957283, 0.6580458), vec2(1.1809901, 0.6168119), vec2(1.1652521, 0.57530063), vec2(1.1486974, 0.53383684), vec2(1.1315429, 0.49270663), vec2(1.1140361, 0.45215553), vec2(1.0964522, 0.41238713), vec2(1.0790933, 0.3735637), vec2(1.0622866, 0.3358072), vec2(1.0463861, 0.29920104), vec2(1.0317768, 0.26379228), vec2(1.018882, 0.22959378), vec2(1.0081772, 0.19658639), vec2(1.0002072, 0.16472021), vec2(0.995619, 0.13391595), vec2(0.9952134, 0.10406538), vec2(1.0, 0.07502907), vec2(1.3887436, 0.99999726), vec2(1.3887115, 0.9995677), vec2(1.388485, 0.99718046), vec2(1.3878709, 0.99190927), vec2(1.3866783, 0.98321545), vec2(1.3847206, 0.97083557), vec2(1.3818164, 0.9546938), vec2(1.3777946, 0.93485534), vec2(1.3725015, 0.9114951), vec2(1.3658037, 0.884871), vec2(1.357593, 0.85530144), vec2(1.3477902, 0.8231462), vec2(1.3363502, 0.78879094), vec2(1.3232652, 0.75263214), vec2(1.3085678, 0.7150653), vec2(1.2923307, 0.6764737), vec2(1.2746682, 0.6372196), vec2(1.255733, 0.5976375), vec2(1.2357132, 0.5580278), vec2(1.2148287, 0.51865375), vec2(1.1933256, 0.47973844), vec2(1.171471, 0.4414642), vec2(1.1495482, 0.40397245), vec2(1.1278509, 0.36736453), vec2(1.1066804, 0.3317033), vec2(1.0863434, 0.29701528), vec2(1.0671506, 0.2632922), vec2(1.0494202, 0.2304932), vec2(1.0334786, 0.19854537), vec2(1.01967, 0.1673446), vec2(1.008373, 0.13675492), vec2(1.0, 0.10660392), vec2(1.486756, 0.99999917), vec2(1.486719, 0.9995904), vec2(1.4864594, 0.99731797), vec2(1.4857557, 0.99229723), vec2(1.4843888, 0.9840122), vec2(1.4821445, 0.97220796), vec2(1.4788146, 0.9568083), vec2(1.4742025, 0.9378713), vec2(1.468131, 0.9155599), vec2(1.4604456, 0.8901164), vec2(1.4510196, 0.86184096), vec2(1.4397587, 0.83107394), vec2(1.4266063, 0.7981808), vec2(1.4115471, 0.76353806), vec2(1.3946096, 0.72752154), vec2(1.3758663, 0.6904957), vec2(1.3554331, 0.6528053), vec2(1.3334674, 0.6147682), vec2(1.3101612, 0.5766695), vec2(1.285738, 0.5387583), vec2(1.260445, 0.50124484), vec2(1.2345456, 0.46429974), vec2(1.208313, 0.4280539), vec2(1.1820227, 0.39259925), vec2(1.1559463, 0.35799047), vec2(1.1303483, 0.32424733), vec2(1.1054804, 0.29135597), vec2(1.0815829, 0.25927186), vec2(1.0588789, 0.22791994), vec2(1.0375794, 0.19719617), vec2(1.0178903, 0.1669673), vec2(0.99999994, 0.1370667), vec2(1.5707864, 1.0), vec2(1.5707451, 0.9996081), vec2(1.5704553, 0.9974216), vec2(1.5696698, 0.99258894), vec2(1.568144, 0.984611), vec2(1.5656384, 0.97323954), vec2(1.5619204, 0.9583983), vec2(1.5567701, 0.9401404), vec2(1.5499887, 0.9186204), vec2(1.5414026, 0.89406943), vec2(1.5308679, 0.86677456), vec2(1.5182765, 0.83706254), vec2(1.5035611, 0.8052845), vec2(1.4866992, 0.771803), vec2(1.4677155, 0.73698044), vec2(1.4466817, 0.7011686), vec2(1.4237157, 0.66470045), vec2(1.3989786, 0.62788343), vec2(1.3726673, 0.59099317), vec2(1.3450104, 0.55427074), vec2(1.3162589, 0.5179198), vec2(1.2866778, 0.48210537), vec2(1.2565396, 0.44695467), vec2(1.226114, 0.41255736), vec2(1.1956619, 0.3789674), vec2(1.1654321, 0.34620595), vec2(1.1356524, 0.31426248), vec2(1.1065328, 0.28309828), vec2(1.0782562, 0.2526468), vec2(1.0509844, 0.22281604), vec2(1.0248611, 0.19348915), vec2(0.99999976, 0.16452146));

layout(set=2,binding=1,std140) uniform PBRUniforms_block_0Fragment { PBRUniforms _group_0_binding_0_fs; };

layout(set=0,binding=0) uniform sampler2D tex;

layout(set=1,binding=0) uniform sampler2D normalTex;

layout(set=3,binding=0) uniform sampler2D metallicRoughnessTex;

layout(set=3,binding=1) uniform sampler2D occlusionTex;

layout(set=3,binding=2) uniform sampler2D emissiveTex;

layout(set=3,binding=3) uniform sampler2D specularTex;

layout(set=3,binding=4) uniform sampler2D specularColorTex;

layout(set=3,binding=5) uniform sampler2D environmentTex;

layout(set=3,binding=6) uniform sampler2D shadowTex;

layout(set=3,binding=7) uniform sampler2D screenDepthTex;

layout(set=3,binding=8) uniform sampler2D clearcoatTex;

layout(set=3,binding=9) uniform sampler2D clearcoatRoughnessTex;

layout(set=3,binding=10) uniform sampler2D clearcoatNormalTex;

layout(set=3,binding=11) uniform sampler2D sheenColorTex;

layout(set=3,binding=12) uniform sampler2D sheenRoughnessTex;

layout(set=3,binding=13) uniform sampler2D anisotropyTex;

layout(location=0) smooth in vec3 _vs2fs_location0;
layout(location=1) smooth in vec3 _vs2fs_location1;
layout(location=2) smooth in vec2 _vs2fs_location2;
layout(location=3) smooth in vec4 _vs2fs_location3;
layout(location=4) smooth in vec4 _vs2fs_location4;
layout(location=5) flat in vec4 _vs2fs_location5;
layout(location=6) flat in vec4 _vs2fs_location6;
layout(location=7) flat in vec4 _vs2fs_location7;
layout(location=8) flat in vec4 _vs2fs_location8;
layout(location=9) flat in vec4 _vs2fs_location9;
layout(location=10) smooth in vec4 _vs2fs_location10;
layout(location = 0) out vec4 _fs2p_location0;

vec3 pbr_srgb(vec3 linear) {
    vec3 c = max(linear, vec3(0.0));
    return mix(((1.055 * pow(c, vec3(0.41666666))) - vec3(0.055)), (12.92 * c), lessThanEqual(c, vec3(0.0031308)));
}

vec3 pbr_tone_map(vec3 linear_1) {
    float _e4 = _group_0_binding_0_fs.renderParams.x;
    float _e8 = _group_0_binding_0_fs.renderParams.x;
    float exposure = ((_e8 <= 0.0) ? 1.0 : _e4);
    vec3 hdr = clamp((linear_1 * exposure), vec3(0.0), vec3(1e20));
    vec3 reinhard = (hdr / (hdr + vec3(1.0)));
    float _e27 = _group_0_binding_0_fs.lightShadows[7].w;
    if ((_e27 < 0.5)) {
        return reinhard;
    }
    mat3x3 inputMatrix = mat3x3(vec3(0.59719, 0.076, 0.0284), vec3(0.35458, 0.90834, 0.13383), vec3(0.04823, 0.01566, 0.83777));
    mat3x3 outputMatrix = mat3x3(vec3(1.60475, -0.10208, -0.00327), vec3(-0.53108, 1.10813, -0.07276), vec3(-0.07367, -0.00605, 1.07602));
    vec3 aces = (inputMatrix * (hdr / vec3(0.6)));
    vec3 reciprocal = (vec3(1.0) / (aces + vec3(1.0)));
    vec3 normalized = (aces * reciprocal);
    vec3 squared = (normalized * normalized);
    vec3 crossTerm = (normalized * reciprocal);
    vec3 fit = (((squared + (0.0245786 * crossTerm)) - ((9.0537e-5 * reciprocal) * reciprocal)) / (((0.983729 * squared) + (0.432951 * crossTerm)) + ((0.238081 * reciprocal) * reciprocal)));
    return clamp((outputMatrix * fit), vec3(0.0), vec3(1.0));
}

vec4 pbr_encode_radiance(vec3 linear_2) {
    vec3 c_1 = max(linear_2, vec3(0.0));
    float exponent = clamp(ceil(log2(max(max(max(c_1.x, c_1.y), c_1.z), 1e-30))), -120.0, 120.0);
    return vec4((c_1 * exp2(-(exponent))), ((exponent + 128.0) / 255.0));
}

vec4 pbr_output(vec3 linear_3, float alpha) {
    float _e5 = _group_0_binding_0_fs.renderParams.y;
    if ((_e5 == 3.0)) {
        vec4 _e8 = pbr_encode_radiance(linear_3);
        return _e8;
    }
    vec3 _e9 = pbr_tone_map(linear_3);
    vec3 _e10 = pbr_srgb(_e9);
    return vec4((_e10 * alpha), alpha);
}

vec3 pbr_fog_color() {
    float _e4 = _group_0_binding_0_fs.lightShadows[0].w;
    float _e9 = _group_0_binding_0_fs.lightShadows[1].w;
    float _e14 = _group_0_binding_0_fs.lightShadows[2].w;
    return vec3(_e4, _e9, _e14);
}

float pbr_fog_factor(vec3 position) {
    float mode = _group_0_binding_0_fs.lightShadows[6].w;
    if ((mode < 0.5)) {
        return 0.0;
    }
    vec4 _e11 = _group_0_binding_0_fs.cameraPosition;
    vec3 delta = (position - _e11.xyz);
    vec4 _e17 = _group_0_binding_0_fs.cameraForward;
    float _e25 = _group_0_binding_0_fs.cameraPosition.w;
    float distance_ = ((_e25 < 0.0) ? max(dot(delta, _e17.xyz), 0.0) : length(delta));
    float start = _group_0_binding_0_fs.lightShadows[3].w;
    if ((mode < 1.5)) {
        float _e41 = _group_0_binding_0_fs.lightShadows[4].w;
        return clamp(((distance_ - start) / max((_e41 - start), 1e-6)), 0.0, 1.0);
    }
    float _e56 = _group_0_binding_0_fs.lightShadows[5].w;
    float opticalDepth = (max((distance_ - start), 0.0) * _e56);
    return (1.0 - exp((-(opticalDepth) * opticalDepth)));
}

vec4 pbr_surface_output(vec3 linear_4, float alpha_1, vec3 position_1) {
    float _e6 = _group_0_binding_0_fs.renderParams.y;
    if ((_e6 == 3.0)) {
        vec4 _e9 = pbr_encode_radiance(linear_4);
        return _e9;
    }
    vec3 _e10 = pbr_fog_color();
    float _e11 = pbr_fog_factor(position_1);
    vec4 _e13 = pbr_output(mix(linear_4, _e10, _e11), alpha_1);
    return _e13;
}

vec4 pbr_decode_radiance(vec4 encoded) {
    return vec4((encoded.xyz * exp2((roundEven((encoded.w * 255.0)) - 128.0))), ((encoded.w > 0.0) ? 1.0 : 0.0));
}

vec4 pbr_atlas(vec2 uv, float level) {
    float _e5 = _group_0_binding_0_fs.environment.z;
    float _e9 = _group_0_binding_0_fs.environment.w;
    vec2 inset = vec2(_e5, _e9);
    vec2 local_41 = ((clamp(uv, vec2(0.0), vec2(1.0)) * (vec2(1.0) - (2.0 * inset))) + inset);
    vec4 _e32 = textureLod(environmentTex, vec2(vec2(local_41.x, ((local_41.y + level) / 8.0))), 0.0);
    return _e32;
}

vec3 pbr_rgbe_atlas(vec2 uv_1, float level_1) {
    ivec2 dims_1 = ivec2(uvec2(textureSize(environmentTex, 0).xy));
    float _e8 = _group_0_binding_0_fs.environment.z;
    float _e12 = _group_0_binding_0_fs.environment.w;
    vec2 inset_1 = vec2(_e8, _e12);
    vec2 local_42 = ((clamp(uv_1, vec2(0.0), vec2(1.0)) * (vec2(1.0) - (2.0 * inset_1))) + inset_1);
    vec2 coord = ((vec2(local_42.x, ((local_42.y + level_1) / 8.0)) * vec2(dims_1)) - vec2(0.5));
    ivec2 base_1 = ivec2(floor(coord));
    vec2 weight_2 = fract(coord);
    int tileHeight = (dims_1.y / 8);
    ivec2 first = ivec2(0, (int(level_1) * tileHeight));
    ivec2 last = ivec2((dims_1.x - 1), ((first.y + tileHeight) - 1));
    vec4 _e58 = texelFetch(environmentTex, min(max(base_1, first), last), 0);
    vec4 _e59 = pbr_decode_radiance(_e58);
    vec3 a = _e59.xyz;
    vec4 _e68 = texelFetch(environmentTex, min(max((base_1 + ivec2(1, 0)), first), last), 0);
    vec4 _e69 = pbr_decode_radiance(_e68);
    vec3 b_1 = _e69.xyz;
    vec4 _e78 = texelFetch(environmentTex, min(max((base_1 + ivec2(0, 1)), first), last), 0);
    vec4 _e79 = pbr_decode_radiance(_e78);
    vec3 c_2 = _e79.xyz;
    vec4 _e87 = texelFetch(environmentTex, min(max((base_1 + ivec2(1)), first), last), 0);
    vec4 _e88 = pbr_decode_radiance(_e87);
    vec3 d = _e88.xyz;
    return mix(mix(a, b_1, weight_2.x), mix(c_2, d, weight_2.x), weight_2.y);
}

vec2 pbr_environment_uv(vec3 direction_1) {
    float _e11 = _group_0_binding_0_fs.environment.y;
    return vec2(fract((((atan(direction_1.z, direction_1.x) / 6.2831855) + 0.5) + (_e11 / 6.2831855))), (acos(clamp(direction_1.y, -1.0, 1.0)) / PI));
}

vec3 pbr_panorama(vec3 direction_2) {
    ivec2 dims_2 = ivec2(uvec2(textureSize(tex, 0).xy));
    vec2 _e4 = pbr_environment_uv(direction_2);
    vec2 coord_1 = ((_e4 * vec2(dims_2)) - vec2(0.5));
    ivec2 origin = ivec2(floor(coord_1));
    vec2 weight_3 = fract(coord_1);
    int x0_ = ((origin.x + dims_2.x) % dims_2.x);
    int x1_ = ((x0_ + 1) % dims_2.x);
    int y0_ = min(max(origin.y, 0), (dims_2.y - 1));
    int y1_ = min(max((origin.y + 1), 0), (dims_2.y - 1));
    vec4 _e39 = texelFetch(tex, ivec2(x0_, y0_), 0);
    vec4 _e40 = pbr_decode_radiance(_e39);
    vec3 a_1 = _e40.xyz;
    vec4 _e45 = texelFetch(tex, ivec2(x1_, y0_), 0);
    vec4 _e46 = pbr_decode_radiance(_e45);
    vec3 b_2 = _e46.xyz;
    vec4 _e51 = texelFetch(tex, ivec2(x0_, y1_), 0);
    vec4 _e52 = pbr_decode_radiance(_e51);
    vec3 c_3 = _e52.xyz;
    vec4 _e57 = texelFetch(tex, ivec2(x1_, y1_), 0);
    vec4 _e58 = pbr_decode_radiance(_e57);
    vec3 d_1 = _e58.xyz;
    return mix(mix(a_1, b_2, weight_3.x), mix(c_3, d_1, weight_3.x), weight_3.y);
}

vec3 pbr_environment(vec3 direction_3, float level_2) {
    vec2 _e2 = pbr_environment_uv(direction_3);
    float _e7 = _group_0_binding_0_fs.lightShadows[8].w;
    if ((_e7 > 0.5)) {
        float lower = floor(level_2);
        float upper = ceil(level_2);
        vec3 _e12 = pbr_rgbe_atlas(_e2, lower);
        if ((lower == upper)) {
            return _e12;
        }
        vec3 _e14 = pbr_rgbe_atlas(_e2, upper);
        return mix(_e12, _e14, fract(level_2));
    }
    vec4 _e18 = pbr_atlas(_e2, floor(level_2));
    vec4 _e20 = pbr_atlas(_e2, ceil(level_2));
    return (mix((_e18.xyz * _e18.w), (_e20.xyz * _e20.w), fract(level_2)) * 16.0);
}

float pbr_shadow_map(uint index_1, vec3 position_2, vec3 normal) {
    vec2 gradient = vec2(0.0);
    bool local_17 = false;
    bool local_18 = false;
    bool local_19 = false;
    float visibility = 0.0;
    int y = 0;
    int x = 0;
    vec4 params = _group_0_binding_0_fs.shadowParams[index_1];
    mat4x4 _e10 = _group_0_binding_0_fs.shadowMatrices[index_1];
    vec4 clip_1 = (_e10 * vec4((position_2 + (normal * params.y)), 1.0));
    if ((clip_1.w <= 0.0)) {
        return 1.0;
    }
    vec3 ndc = (clip_1.xyz / vec3(clip_1.w));
    vec2 localUV = vec2(((ndc.x * 0.5) + 0.5), (SHADOW_FLIP_Y ? (0.5 - (ndc.y * 0.5)) : ((ndc.y * 0.5) + 0.5)));
    vec4 rect = _group_0_binding_0_fs.shadowRects[index_1];
    vec2 origin_1 = vec2(rect.x, (SHADOW_FLIP_Y ? rect.y : ((1.0 - rect.y) - rect.w)));
    vec2 uv_5 = (origin_1 + (localUV * rect.zw));
    ivec2 dims_3 = ivec2(uvec2(textureSize(shadowTex, 0).xy));
    ivec2 tileMin = ivec2(roundEven((origin_1 * vec2(dims_3))));
    ivec2 tileMax = ((tileMin + ivec2(roundEven((rect.zw * vec2(dims_3))))) - ivec2(1));
    vec2 texel = ((uv_5 * vec2(dims_3)) - vec2(0.5));
    vec2 base_2 = floor(texel);
    vec2 fraction = (texel - base_2);
    vec3 dx = dFdx(vec3(texel, ndc.z));
    vec3 dy = dFdy(vec3(texel, ndc.z));
    float determinant_ = ((dx.x * dy.y) - (dx.y * dy.x));
    if ((abs(determinant_) > 1e-8)) {
        gradient = (vec2(((dy.y * dx.z) - (dx.y * dy.z)), ((dx.x * dy.z) - (dy.x * dx.z))) / vec2(determinant_));
    }
    if (!(any(lessThan(localUV, vec2(0.0))))) {
        local_17 = any(greaterThan(localUV, vec2(1.0)));
    } else {
        local_17 = true;
    }
    bool _e131 = local_17;
    if (!(_e131)) {
        local_18 = (ndc.z < 0.0);
    } else {
        local_18 = true;
    }
    bool _e139 = local_18;
    if (!(_e139)) {
        local_19 = (ndc.z > 1.0);
    } else {
        local_19 = true;
    }
    bool _e147 = local_19;
    if (_e147) {
        return 1.0;
    }
    vec4 weightsX = vec4((1.0 - fraction.x), 1.0, 1.0, fraction.x);
    vec4 weightsY = vec4((1.0 - fraction.y), 1.0, 1.0, fraction.y);
    bool loop_init = true;
    while(true) {
        if (!loop_init) {
            int _e222 = y;
            y = (_e222 + 1);
        }
        loop_init = false;
        int _e167 = y;
        if ((_e167 < 4)) {
        } else {
            break;
        }
        {
            x = 0;
            bool loop_init_1 = true;
            while(true) {
                if (!loop_init_1) {
                    int _e219 = x;
                    x = (_e219 + 1);
                }
                loop_init_1 = false;
                int _e172 = x;
                if ((_e172 < 4)) {
                } else {
                    break;
                }
                {
                    int _e176 = x;
                    int _e179 = y;
                    ivec2 pixel_1 = min(max((ivec2(base_2) + ivec2((_e176 - 1), (_e179 - 1))), tileMin), tileMax);
                    vec4 _e187 = texelFetch(shadowTex, pixel_1, 0);
                    vec3 packed_ = _e187.xyz;
                    float depth = ((dot(packed_, vec3(65536.0, 256.0, 1.0)) * 255.0) / 16777215.0);
                    vec2 _e199 = gradient;
                    float receiverDepth = ((ndc.z + dot(_e199, (vec2(pixel_1) - texel))) + params.x);
                    float _e206 = visibility;
                    int _e207 = x;
                    int _e209 = y;
                    visibility = (_e206 + ((weightsX[_e207] * weightsY[_e209]) * ((receiverDepth >= depth) ? 1.0 : 0.0)));
                }
            }
        }
    }
    float _e224 = visibility;
    return (_e224 / 9.0);
}

float pbr_light_shadow(uint light, vec3 position_3, vec3 normal_1) {
    uint face = 0u;
    bool local_20 = false;
    bool local_21 = false;
    uint cascade = 0u;
    float next = 0.0;
    bool local_22 = false;
    vec4 mapping = _group_0_binding_0_fs.lightShadows[light];
    if ((mapping.x < 0.5)) {
        return 1.0;
    }
    uint first_1 = (uint(mapping.x) - 1u);
    uint count = uint(mapping.y);
    if ((count == 1u)) {
        float _e19 = pbr_shadow_map(first_1, position_3, normal_1);
        return _e19;
    }
    float _e24 = _group_0_binding_0_fs.lightDirections[light].w;
    if ((_e24 == 2.0)) {
        vec4 _e30 = _group_0_binding_0_fs.lightPosition[light];
        vec3 delta_1 = (position_3 - _e30.xyz);
        vec3 axis_1 = abs(delta_1);
        face = ((delta_1.x >= 0.0) ? 0u : 1u);
        if ((axis_1.y > axis_1.x)) {
            local_20 = (axis_1.y >= axis_1.z);
        } else {
            local_20 = false;
        }
        bool _e50 = local_20;
        if (_e50) {
            face = ((delta_1.y >= 0.0) ? 2u : 3u);
        } else {
            if ((axis_1.z > axis_1.x)) {
                local_21 = (axis_1.z > axis_1.y);
            } else {
                local_21 = false;
            }
            bool _e66 = local_21;
            if (_e66) {
                face = ((delta_1.z >= 0.0) ? 4u : 5u);
            }
        }
        uint _e73 = face;
        float _e75 = pbr_shadow_map((first_1 + _e73), position_3, normal_1);
        return _e75;
    }
    vec4 _e78 = _group_0_binding_0_fs.cameraPosition;
    vec4 _e83 = _group_0_binding_0_fs.cameraForward;
    float distance_1 = dot((position_3 - _e78.xyz), _e83.xyz);
    float _e90 = _group_0_binding_0_fs.shadowParams[first_1].z;
    if ((distance_1 < _e90)) {
        return 1.0;
    }
    bool loop_init_2 = true;
    while(true) {
        if (!loop_init_2) {
            uint _e138 = cascade;
            cascade = (_e138 + 1u);
        }
        loop_init_2 = false;
        uint _e95 = cascade;
        if ((_e95 < 4u)) {
        } else {
            break;
        }
        {
            uint _e98 = cascade;
            if ((_e98 >= count)) {
                break;
            }
            uint _e100 = cascade;
            uint index_4 = (first_1 + _e100);
            vec4 params_1 = _group_0_binding_0_fs.shadowParams[index_4];
            if ((distance_1 <= params_1.w)) {
                float _e108 = pbr_shadow_map(index_4, position_3, normal_1);
                float blendWidth = ((params_1.w - params_1.z) * mapping.z);
                if ((blendWidth <= 0.0)) {
                    return _e108;
                }
                float blend = smoothstep((params_1.w - blendWidth), params_1.w, distance_1);
                next = 1.0;
                uint _e122 = cascade;
                if (((_e122 + 1u) < count)) {
                    local_22 = (blend > 0.0);
                } else {
                    local_22 = false;
                }
                bool _e131 = local_22;
                if (_e131) {
                    float _e134 = pbr_shadow_map((index_4 + 1u), position_3, normal_1);
                    next = _e134;
                }
                float _e135 = next;
                return mix(_e108, _e135, blend);
            }
        }
    }
    return 1.0;
}

vec3 pbr_occlusion_clip(vec3 position_4) {
    mat4x4 m = _group_0_binding_0_fs.occlusionInverseProjection;
    vec3 a_2 = (m[0].xyz - (position_4 * m[0].w));
    vec3 b_3 = (m[1].xyz - (position_4 * m[1].w));
    vec3 c_4 = (m[2].xyz - (position_4 * m[2].w));
    vec3 d_2 = ((position_4 * m[3].w) - m[3].xyz);
    float determinant_1 = dot(a_2, cross(b_3, c_4));
    return (vec3(dot(d_2, cross(b_3, c_4)), dot(a_2, cross(d_2, c_4)), dot(a_2, cross(b_3, d_2))) / vec3(determinant_1));
}

float pbr_screen_occlusion(vec3 position_5, vec3 normal_2) {
    bool local_23 = false;
    float blocked = 0.0;
    uint sampleIndex = 0u;
    bool local_24 = false;
    float radius_1 = _group_0_binding_0_fs.lightMeta.z;
    float strength = _group_0_binding_0_fs.renderParams.z;
    if (!((strength <= 0.0))) {
        local_23 = (radius_1 <= 0.0);
    } else {
        local_23 = true;
    }
    bool _e18 = local_23;
    if (_e18) {
        return 1.0;
    }
    mat4x4 m_1 = _group_0_binding_0_fs.occlusionInverseProjection;
    vec3 _e23 = pbr_occlusion_clip(position_5);
    vec2 uv_6 = vec2(((_e23.x * 0.5) + 0.5), (SHADOW_FLIP_Y ? (0.5 - (_e23.y * 0.5)) : ((_e23.y * 0.5) + 0.5)));
    vec2 dims_4 = vec2(uvec2(textureSize(screenDepthTex, 0).xy));
    vec4 homogeneous = (m_1 * vec4(_e23, 1.0));
    vec2 axes = vec2(length((m_1[0].xyz - (position_5 * m_1[0].w))), length((m_1[1].xyz - (position_5 * m_1[1].w))));
    vec2 denominator = (axes - (radius_1 * vec2(m_1[0].w, m_1[1].w)));
    vec2 extent_1 = max((abs((vec2((radius_1 * homogeneous.w)) / denominator)) * 0.5), (vec2(1.0) / dims_4));
    float _e84 = _group_0_binding_0_fs.cameraForward.w;
    uint count_1 = uint(clamp(_e84, 8.0, 32.0));
    float diskScale = inversesqrt(float(count_1));
    bool loop_init_3 = true;
    while(true) {
        if (!loop_init_3) {
            uint _e193 = sampleIndex;
            sampleIndex = (_e193 + 1u);
        }
        loop_init_3 = false;
        uint _e95 = sampleIndex;
        if ((_e95 < 32u)) {
        } else {
            break;
        }
        {
            uint _e98 = sampleIndex;
            if ((_e98 >= count_1)) {
                break;
            }
            uint _e101 = sampleIndex;
            vec2 sampleUV = (uv_6 + ((SSAO_DISK[_e101] * diskScale) * extent_1));
            if (!(any(lessThan(sampleUV, vec2(0.0))))) {
                local_24 = any(greaterThanEqual(sampleUV, vec2(1.0)));
            } else {
                local_24 = true;
            }
            bool _e118 = local_24;
            if (_e118) {
                continue;
            }
            ivec2 pixel_2 = ivec2((sampleUV * dims_4));
            vec4 packed_1 = texelFetch(screenDepthTex, pixel_2, 0);
            if ((packed_1.w < 0.5)) {
                continue;
            }
            float depth_1 = ((dot(packed_1.xyz, vec3(65536.0, 256.0, 1.0)) * 255.0) / 16777215.0);
            vec2 center = ((vec2(pixel_2) + vec2(0.5)) / dims_4);
            vec3 ndc_1 = vec3(((center.x * 2.0) - 1.0), (SHADOW_FLIP_Y ? (1.0 - (center.y * 2.0)) : ((center.y * 2.0) - 1.0)), depth_1);
            vec4 world = (m_1 * vec4(ndc_1, 1.0));
            if ((abs(world.w) < 1e-8)) {
                continue;
            }
            vec3 delta_2 = ((world.xyz / vec3(world.w)) - position_5);
            float distance_2 = length(delta_2);
            float _e177 = _group_0_binding_0_fs.lightMeta.w;
            float horizon = (max((dot(normal_2, delta_2) - _e177), 0.0) / max(distance_2, 1e-6));
            float _e184 = blocked;
            blocked = (_e184 + (horizon * (1.0 - smoothstep((radius_1 * 0.5), radius_1, distance_2))));
        }
    }
    float _e197 = blocked;
    return clamp((1.0 - (((strength * 2.0) * _e197) / float(count_1))), 0.0, 1.0);
}

vec3 pbr_fresnel(float cosine, vec3 f0_, float f90_) {
    return (f0_ + ((vec3(f90_) - f0_) * pow((1.0 - clamp(cosine, 0.0, 1.0)), 5.0)));
}

vec2 pbr_sheen_response(float nv, float roughness_1) {
    vec2 coord_2 = vec2((pow(clamp(nv, 0.0, 1.0), 0.33333334) * 31.0), (sqrt(((clamp(roughness_1, 0.045, 1.0) - 0.045) / 0.955)) * 15.0));
    uvec2 cell = uvec2(min(floor(coord_2), vec2(30.0, 14.0)));
    vec2 weight_4 = (coord_2 - vec2(cell));
    uint i = ((cell.y * 32u) + cell.x);
    return mix(mix(SHEEN_ALBEDO[i], SHEEN_ALBEDO[(i + 1u)], weight_4.x), mix(SHEEN_ALBEDO[(i + 32u)], SHEEN_ALBEDO[(i + 33u)], weight_4.x), weight_4.y);
}

float pbr_ggx(float nl, float nv_1, float nh, float roughness_2) {
    float a_3 = (roughness_2 * roughness_2);
    float a2_ = (a_3 * a_3);
    float denominator_1 = (((nh * nh) * (a2_ - 1.0)) + 1.0);
    float d_3 = (a2_ / max(((PI * denominator_1) * denominator_1), 1e-7));
    float gv = (nl * sqrt((((nv_1 * nv_1) * (1.0 - a2_)) + a2_)));
    float maskingLight = (nv_1 * sqrt((((nl * nl) * (1.0 - a2_)) + a2_)));
    return ((d_3 * 0.5) / max((gv + maskingLight), 0.0001));
}

vec3 pbr_reflection_normal(PBRSurface s, vec3 v) {
    if ((s.anisotropy > 0.0)) {
        vec3 projected = (v - (s.bitangent * dot(v, s.bitangent)));
        if ((dot(projected, projected) > 1e-8)) {
            float stretch = (s.anisotropy * (1.0 - s.roughness));
            return normalize(mix(s.n, normalize(projected), (((stretch * stretch) * stretch) * stretch)));
        }
    }
    return s.n;
}

vec2 pbr_screen_uv(vec4 clip) {
    vec2 ndc_2 = (clip.xy / vec2(clip.w));
    return vec2(((ndc_2.x * 0.5) + 0.5), (SHADOW_FLIP_Y ? (0.5 - (ndc_2.y * 0.5)) : ((ndc_2.y * 0.5) + 0.5)));
}

ivec2 pbr_scene_dimensions() {
    ivec2 dims = ivec2(0);
    dims = ivec2(uvec2(textureSize(shadowTex, 0).xy));
    float _e8 = _group_0_binding_0_fs.shadowRects[0].z;
    if ((_e8 > 0.5)) {
        int _e12 = dims.x;
        dims.x = (_e12 / 2);
    }
    ivec2 _e15 = dims;
    return _e15;
}

vec4 pbr_scene_color(vec2 uv_2) {
    ivec2 _e1 = pbr_scene_dimensions();
    vec2 coord_3 = ((uv_2 * vec2(_e1)) - vec2(0.5));
    ivec2 base_3 = ivec2(floor(coord_3));
    vec2 weight_5 = fract(coord_3);
    ivec2 last_1 = (_e1 - ivec2(1));
    vec4 _e18 = texelFetch(shadowTex, min(max(base_3, ivec2(0)), last_1), 0);
    vec4 _e19 = pbr_decode_radiance(_e18);
    vec4 _e29 = texelFetch(shadowTex, min(max((base_3 + ivec2(1, 0)), ivec2(0)), last_1), 0);
    vec4 _e30 = pbr_decode_radiance(_e29);
    vec4 _e40 = texelFetch(shadowTex, min(max((base_3 + ivec2(0, 1)), ivec2(0)), last_1), 0);
    vec4 _e41 = pbr_decode_radiance(_e40);
    vec4 _e50 = texelFetch(shadowTex, min(max((base_3 + ivec2(1)), ivec2(0)), last_1), 0);
    vec4 _e51 = pbr_decode_radiance(_e50);
    return mix(mix(_e19, _e30, weight_5.x), mix(_e41, _e51, weight_5.x), weight_5.y);
}

vec2 pbr_scene_depth(vec2 uv_3) {
    ivec2 dims_5 = ivec2(uvec2(textureSize(screenDepthTex, 0).xy));
    ivec2 pixel_3 = min(max(ivec2((uv_3 * vec2(dims_5))), ivec2(0)), (dims_5 - ivec2(1)));
    vec4 packed_2 = texelFetch(screenDepthTex, pixel_3, 0);
    if ((packed_2.w < 0.5)) {
        return vec2(0.0);
    }
    float depth_2 = ((dot(packed_2.xyz, vec3(65536.0, 256.0, 1.0)) * 255.0) / 16777215.0);
    vec2 center_1 = ((vec2(pixel_3) + vec2(0.5)) / vec2(dims_5));
    mat4x4 _e39 = _group_0_binding_0_fs.occlusionInverseProjection;
    vec4 world_1 = (_e39 * vec4(((center_1.x * 2.0) - 1.0), (SHADOW_FLIP_Y ? (1.0 - (center_1.y * 2.0)) : ((center_1.y * 2.0) - 1.0)), depth_2, 1.0));
    if ((abs(world_1.w) < 1e-8)) {
        return vec2(0.0);
    }
    vec4 _e72 = _group_0_binding_0_fs.cameraPosition;
    vec4 _e77 = _group_0_binding_0_fs.cameraForward;
    return vec2(dot(((world_1.xyz / vec3(world_1.w)) - _e72.xyz), _e77.xyz), 1.0);
}

vec4 pbr_screen_reflection(vec3 position_6, vec3 normal_3, vec3 v_1, float roughness_3) {
    bool local_25 = false;
    vec3 end = vec3(0.0);
    vec4 b = vec4(0.0);
    bool local_26 = false;
    bool local_27 = false;
    float extent = 1.0;
    bool local_28 = false;
    float limit = 1.0;
    uint axis = 0u;
    float previous = 0.0;
    float previousDelta = 0.0;
    uint index_2 = 1u;
    bool local_29 = false;
    bool local_30 = false;
    bool local_31 = false;
    float lo = 0.0;
    float hi = 0.0;
    uint refine = 0u;
    bool local_32 = false;
    bool local_33 = false;
    bool local_34 = false;
    bool local_35 = false;
    bool local_36 = false;
    vec4 settings = _group_0_binding_0_fs.shadowParams[0];
    float maxDistance = settings.x;
    float thickness = settings.y;
    if (!((roughness_3 >= settings.w))) {
        local_25 = (settings.z <= 0.0);
    } else {
        local_25 = true;
    }
    bool _e19 = local_25;
    if (_e19) {
        return vec4(0.0);
    }
    vec3 ray = reflect(-(v_1), normal_3);
    vec3 start_1 = (position_6 + (normal_3 * (thickness * 0.25)));
    end = (start_1 + (ray * maxDistance));
    mat4x4 projection = _group_0_binding_0_fs.shadowMatrices[0];
    vec4 a_4 = (projection * vec4(start_1, 1.0));
    vec3 _e38 = end;
    b = (projection * vec4(_e38, 1.0));
    if (!((a_4.w <= 1e-6))) {
        local_26 = (a_4.z < 0.0);
    } else {
        local_26 = true;
    }
    bool _e53 = local_26;
    if (!(_e53)) {
        local_27 = (a_4.z > a_4.w);
    } else {
        local_27 = true;
    }
    bool _e61 = local_27;
    if (_e61) {
        return vec4(0.0);
    }
    float _e67 = b.w;
    float _e69 = b.z;
    if (((_e67 - _e69) < 1e-6)) {
        float _e73 = extent;
        float _e83 = b.w;
        float _e86 = b.z;
        extent = min(_e73, (((a_4.w - a_4.z) - 1e-6) / max((((a_4.w - a_4.z) - _e83) + _e86), 1e-6)));
    }
    float _e93 = b.z;
    if ((_e93 < 0.0)) {
        float _e96 = extent;
        float _e100 = b.z;
        extent = min(_e96, (a_4.z / max((a_4.z - _e100), 1e-6)));
    }
    vec3 _e106 = end;
    float _e107 = extent;
    end = mix(start_1, _e106, clamp(_e107, 0.0, 1.0));
    vec3 _e112 = end;
    b = (projection * vec4(_e112, 1.0));
    float _e117 = b.w;
    if ((_e117 <= 1e-6)) {
        return vec4(0.0);
    }
    vec2 _e122 = pbr_screen_uv(a_4);
    vec4 _e123 = b;
    vec2 _e124 = pbr_screen_uv(_e123);
    vec2 deltaUV = (_e124 - _e122);
    if (!(any(lessThan(_e122, vec2(0.0))))) {
        local_28 = any(greaterThanEqual(_e122, vec2(1.0)));
    } else {
        local_28 = true;
    }
    bool _e138 = local_28;
    if (_e138) {
        return vec4(0.0);
    }
    bool loop_init_4 = true;
    while(true) {
        if (!loop_init_4) {
            uint _e174 = axis;
            axis = (_e174 + 1u);
        }
        loop_init_4 = false;
        uint _e145 = axis;
        if ((_e145 < 2u)) {
        } else {
            break;
        }
        {
            uint _e148 = axis;
            if ((deltaUV[_e148] > 1e-6)) {
                float _e152 = limit;
                uint _e153 = axis;
                uint _e157 = axis;
                limit = min(_e152, ((1.0 - _e122[_e153]) / deltaUV[_e157]));
            }
            uint _e161 = axis;
            if ((deltaUV[_e161] < -1e-6)) {
                float _e165 = limit;
                uint _e166 = axis;
                uint _e169 = axis;
                limit = min(_e165, (-(_e122[_e166]) / deltaUV[_e169]));
            }
        }
    }
    float _e181 = limit;
    float pixels_1 = (length((deltaUV * vec2(uvec2(textureSize(screenDepthTex, 0).xy)))) * _e181);
    if ((pixels_1 < 2.0)) {
        return vec4(0.0);
    }
    float _e192 = _group_0_binding_0_fs.shadowRects[0].x;
    uint steps = uint(min(ceil(pixels_1), _e192));
    vec3 q0_ = (start_1 / vec3(a_4.w));
    vec3 _e198 = end;
    float _e200 = b.w;
    vec3 q1_ = (_e198 / vec3(_e200));
    float k0_ = (1.0 / a_4.w);
    float _e207 = b.w;
    float k1_ = (1.0 / _e207);
    previousDelta = -(thickness);
    bool loop_init_5 = true;
    while(true) {
        if (!loop_init_5) {
            uint _e417 = index_2;
            index_2 = (_e417 + 1u);
        }
        loop_init_5 = false;
        uint _e216 = index_2;
        if ((_e216 <= 128u)) {
        } else {
            break;
        }
        {
            uint _e219 = index_2;
            if ((_e219 > steps)) {
                break;
            }
            uint _e221 = index_2;
            float _e223 = limit;
            float t_1 = ((float(_e221) * _e223) / float(steps));
            vec2 uv_7 = (_e122 + (deltaUV * t_1));
            if (!(any(lessThanEqual(uv_7, vec2(0.0))))) {
                local_29 = any(greaterThanEqual(uv_7, vec2(1.0)));
            } else {
                local_29 = true;
            }
            bool _e241 = local_29;
            if (_e241) {
                break;
            }
            vec3 point_1 = (mix(q0_, q1_, t_1) / vec3(mix(k0_, k1_, t_1)));
            vec2 _e246 = pbr_scene_depth(uv_7);
            vec4 _e249 = _group_0_binding_0_fs.cameraPosition;
            vec4 _e254 = _group_0_binding_0_fs.cameraForward;
            float difference = (dot((point_1 - _e249.xyz), _e254.xyz) - _e246.x);
            if ((_e246.y > 0.5)) {
                local_30 = (difference >= 0.0);
            } else {
                local_30 = false;
            }
            bool _e267 = local_30;
            if (_e267) {
                float _e270 = previousDelta;
                local_31 = (_e270 < 0.0);
            } else {
                local_31 = false;
            }
            bool _e274 = local_31;
            if (_e274) {
                float _e275 = previous;
                lo = _e275;
                hi = t_1;
                refine = 0u;
                bool loop_init_6 = true;
                while(true) {
                    if (!loop_init_6) {
                        uint _e317 = refine;
                        refine = (_e317 + 1u);
                    }
                    loop_init_6 = false;
                    uint _e280 = refine;
                    if ((_e280 < 6u)) {
                    } else {
                        break;
                    }
                    {
                        float _e283 = lo;
                        float _e284 = hi;
                        float mid = ((_e283 + _e284) * 0.5);
                        vec2 _e290 = pbr_scene_depth((_e122 + (deltaUV * mid)));
                        vec3 midPoint = (mix(q0_, q1_, mid) / vec3(mix(k0_, k1_, mid)));
                        vec4 _e297 = _group_0_binding_0_fs.cameraPosition;
                        vec4 _e302 = _group_0_binding_0_fs.cameraForward;
                        float midDelta = (dot((midPoint - _e297.xyz), _e302.xyz) - _e290.x);
                        if ((_e290.y > 0.5)) {
                            local_32 = (midDelta >= 0.0);
                        } else {
                            local_32 = false;
                        }
                        bool _e315 = local_32;
                        if (_e315) {
                            hi = mid;
                        } else {
                            lo = mid;
                        }
                    }
                }
                float _e319 = hi;
                vec2 hitUV = (_e122 + (deltaUV * _e319));
                float _e322 = hi;
                float _e324 = hi;
                vec3 hitPoint = (mix(q0_, q1_, _e322) / vec3(mix(k0_, k1_, _e324)));
                vec2 _e328 = pbr_scene_depth(hitUV);
                vec4 _e331 = _group_0_binding_0_fs.cameraPosition;
                vec4 _e336 = _group_0_binding_0_fs.cameraForward;
                float hitDelta = (dot((hitPoint - _e331.xyz), _e336.xyz) - _e328.x);
                float distance_3 = length((hitPoint - position_6));
                if ((_e328.y > 0.5)) {
                    local_33 = (hitDelta >= 0.0);
                } else {
                    local_33 = false;
                }
                bool _e351 = local_33;
                if (_e351) {
                    local_34 = (hitDelta <= thickness);
                } else {
                    local_34 = false;
                }
                bool _e356 = local_34;
                if (_e356) {
                    local_35 = (distance_3 > thickness);
                } else {
                    local_35 = false;
                }
                bool _e361 = local_35;
                if (_e361) {
                    float _e364 = hi;
                    float _e366 = limit;
                    local_36 = (((pixels_1 * _e364) / _e366) >= 2.0);
                } else {
                    local_36 = false;
                }
                bool _e371 = local_36;
                if (_e371) {
                    float edge = min(min(hitUV.x, hitUV.y), min((1.0 - hitUV.x), (1.0 - hitUV.y)));
                    float _e387 = _group_0_binding_0_fs.shadowRects[0].y;
                    float fade = (((smoothstep(0.0, _e387, edge) * (1.0 - smoothstep((maxDistance * 0.8), maxDistance, distance_3))) * (1.0 - smoothstep((settings.w * 0.75), settings.w, roughness_3))) * settings.z);
                    vec4 _e406 = pbr_scene_color(hitUV);
                    return vec4(_e406.xyz, (fade * _e406.w));
                }
            }
            previous = t_1;
            previousDelta = ((_e246.y > 0.5) ? difference : -(thickness));
        }
    }
    return vec4(0.0);
}

vec3 pbr_environment_reflection(vec3 normal_4, vec3 v_2, float roughness_4) {
    float roughnessSquared = (roughness_4 * roughness_4);
    vec3 direction_5 = normalize(mix(reflect(-(v_2), normal_4), normal_4, (roughnessSquared * roughnessSquared)));
    vec3 _e11 = pbr_environment(direction_5, (roughness_4 * 5.0));
    return _e11;
}

vec3 pbr_multiscattering(vec3 f0_1, float f90_1, vec2 brdf) {
    vec3 single = ((f0_1 * brdf.x) + vec3((f90_1 * brdf.y)));
    float missing = max(0.0, ((1.0 - brdf.x) - brdf.y));
    vec3 average = (f0_1 + ((vec3(1.0) - f0_1) / vec3(21.0)));
    return (((single * average) * missing) / max(vec3(0.0001), (vec3(1.0) - (average * missing))));
}

vec2 pbr_environment_brdf(float nv_2, float roughness_5) {
    float _e5 = _group_0_binding_0_fs.environment.z;
    if ((_e5 > 0.0)) {
        vec4 _e10 = pbr_atlas(vec2(nv_2, roughness_5), 7.0);
        return _e10.xy;
    }
    vec4 r = ((roughness_5 * vec4(-1.0, -0.0275, -0.572, 0.022)) + vec4(1.0, 0.0425, 1.04, -0.04));
    float a_5 = ((min((r.x * r.x), exp2((-9.28 * nv_2))) * r.x) + r.y);
    return ((vec2(-1.04, 1.04) * a_5) + r.zw);
}

vec3 pbr_reflection_delta(vec3 position_7, vec3 normal_5, vec3 v_3, float roughness_6, vec3 weight) {
    vec3 fallback = vec3(0.0);
    if (!(any(greaterThan(weight, vec3(0.0))))) {
        return vec3(0.0);
    }
    vec4 _e12 = pbr_screen_reflection(position_7, normal_5, v_3, roughness_6);
    if ((_e12.w <= 0.0)) {
        return vec3(0.0);
    }
    float _e24 = _group_0_binding_0_fs.environment.x;
    if ((_e24 > 0.0)) {
        vec3 _e27 = pbr_environment_reflection(normal_5, v_3, roughness_6);
        float _e31 = _group_0_binding_0_fs.environment.x;
        fallback = (_e27 * _e31);
    }
    vec3 _e34 = fallback;
    return (((_e12.xyz - _e34) * weight) * _e12.w);
}

vec4 pbr_resolve_reflections(PBRInput input_1, PBRSurface s_1, vec3 v_4, float materialAO, uint flags) {
    vec3 lit_1 = vec3(0.0);
    float ao_1 = 0.0;
    mat4x4 _e8 = _group_0_binding_0_fs.shadowMatrices[0];
    vec2 _e13 = pbr_screen_uv((_e8 * vec4(input_1.worldPosition, 1.0)));
    ivec2 _e14 = pbr_scene_dimensions();
    ivec2 pixel_4 = min(max(ivec2((_e13 * vec2(_e14))), ivec2(0)), (_e14 - ivec2(1)));
    vec4 _e26 = texelFetch(shadowTex, pixel_4, 0);
    vec4 _e27 = pbr_decode_radiance(_e26);
    lit_1 = _e27.xyz;
    if (((flags & 4194304u) != 0u)) {
        vec3 _e34 = lit_1;
        vec4 _e37 = pbr_surface_output(_e34, 1.0, input_1.worldPosition);
        return _e37;
    }
    ao_1 = materialAO;
    float _e43 = _group_0_binding_0_fs.shadowRects[0].z;
    if ((_e43 > 0.5)) {
        vec4 _e52 = texelFetch(shadowTex, (pixel_4 + ivec2(_e14.x, 0)), 0);
        ao_1 = _e52.x;
    }
    vec2 _e59 = pbr_environment_brdf(max(dot(s_1.n, v_4), 0.0001), s_1.roughness);
    vec3 _e60 = lit_1;
    vec3 _e62 = pbr_reflection_normal(s_1, v_4);
    float _e78 = ao_1;
    vec3 _e80 = pbr_reflection_delta(input_1.worldPosition, _e62, v_4, s_1.roughness, (((((s_1.f0_ * _e59.x) + vec3((s_1.f90_ * _e59.y))) * s_1.sheenScale) * (1.0 - s_1.coatFresnel)) * _e78));
    lit_1 = (_e60 + _e80);
    if (any(greaterThan(s_1.sheen, vec3(0.0)))) {
        vec3 _e87 = lit_1;
        float _e98 = ao_1;
        vec3 _e100 = pbr_reflection_delta(input_1.worldPosition, s_1.n, v_4, s_1.sheenRoughness, (((s_1.sheen * s_1.sheenAlbedo) * (1.0 - s_1.coatFresnel)) * _e98));
        lit_1 = (_e87 + _e100);
    }
    if ((s_1.coat > 0.0)) {
        vec2 _e110 = pbr_environment_brdf(max(dot(s_1.coatNormal, v_4), 0.0001), s_1.coatRoughness);
        vec3 _e111 = lit_1;
        float _e122 = ao_1;
        vec3 _e125 = pbr_reflection_delta(input_1.worldPosition, s_1.coatNormal, v_4, s_1.coatRoughness, vec3(((s_1.coat * ((0.04 * _e110.x) + _e110.y)) * _e122)));
        lit_1 = (_e111 + _e125);
    }
    vec3 _e127 = lit_1;
    vec4 _e130 = pbr_surface_output(_e127, 1.0, input_1.worldPosition);
    return _e130;
}

vec3 pbr_brdf(PBRSurface s_2, vec3 v_5, vec3 l_1) {
    float specular_1 = 0.0;
    vec3 result = vec3(0.0);
    bool local_37 = false;
    float nl_1 = max(dot(s_2.n, l_1), 0.0);
    float nv_3 = max(dot(s_2.n, v_5), 0.0001);
    vec3 halfSum = (v_5 + l_1);
    vec3 h = (halfSum / vec3(max(length(halfSum), 0.0001)));
    float nh_1 = max(dot(s_2.n, h), 0.0);
    float vh = max(dot(v_5, h), 0.0);
    float _e25 = pbr_ggx(nl_1, nv_3, nh_1, s_2.roughness);
    specular_1 = _e25;
    if ((s_2.anisotropy > 0.0)) {
        float ab = (s_2.roughness * s_2.roughness);
        float at = mix(ab, 1.0, (s_2.anisotropy * s_2.anisotropy));
        float ht = dot(h, s_2.tangent);
        float hb = dot(h, s_2.bitangent);
        float denominator_2 = ((((ht * ht) / (at * at)) + ((hb * hb) / (ab * ab))) + (nh_1 * nh_1));
        float d_4 = (1.0 / max(((((PI * at) * ab) * denominator_2) * denominator_2), 1e-7));
        vec3 vv = vec3((at * dot(v_5, s_2.tangent)), (ab * dot(v_5, s_2.bitangent)), nv_3);
        vec3 ll = vec3((at * dot(l_1, s_2.tangent)), (ab * dot(l_1, s_2.bitangent)), nl_1);
        specular_1 = ((d_4 * 0.5) / max(((nl_1 * length(vv)) + (nv_3 * length(ll))), 0.0001));
    }
    vec3 _e86 = pbr_fresnel(vh, s_2.f0_, s_2.f90_);
    float diffuseF = (s_2.diffuseF0_ + ((s_2.diffuseF90_ - s_2.diffuseF0_) * pow((1.0 - clamp(vh, 0.0, 1.0)), 5.0)));
    vec3 diffuse = ((((1.0 - diffuseF) * (1.0 - s_2.metallic)) * s_2.base) / vec3(3.1415927));
    float _e111 = specular_1;
    result = ((diffuse + (_e111 * _e86)) * nl_1);
    if (any(greaterThan(s_2.sheen, vec3(0.0)))) {
        local_37 = (nl_1 > 0.0);
    } else {
        local_37 = false;
    }
    bool _e126 = local_37;
    if (_e126) {
        float alpha_2 = (s_2.sheenRoughness * s_2.sheenRoughness);
        float inverse_ = (1.0 / alpha_2);
        float distribution = (((2.0 + inverse_) * pow(max((1.0 - (nh_1 * nh_1)), 0.0), (0.5 * inverse_))) / 6.2831855);
        vec2 _e146 = pbr_sheen_response(nl_1, s_2.sheenRoughness);
        float maskingLight_1 = min((_e146.x * nl_1), 1.0);
        float gv_1 = s_2.sheenG1_;
        float visibility_1 = (((gv_1 * maskingLight_1) / max(((gv_1 + maskingLight_1) - (gv_1 * maskingLight_1)), 1e-8)) / ((4.0 * nv_3) * nl_1));
        vec3 _e163 = result;
        result = ((_e163 * s_2.sheenScale) + (((s_2.sheen * distribution) * visibility_1) * nl_1));
    }
    if ((s_2.coat > 0.0)) {
        float cnl = max(dot(s_2.coatNormal, l_1), 0.0);
        float cnv = max(dot(s_2.coatNormal, v_5), 0.0001);
        float cnh = max(dot(s_2.coatNormal, h), 0.0);
        vec3 _e186 = result;
        float _e193 = pbr_ggx(cnl, cnv, cnh, s_2.coatRoughness);
        result = ((_e186 * (1.0 - s_2.coatFresnel)) + vec3(((s_2.coatFresnel * _e193) * cnl)));
    }
    vec3 _e198 = result;
    return _e198;
}

mat3x3 pbr_basis(PBRInput input_2, vec3 n_1) {
    vec3 t = vec3(0.0);
    float handedness = 0.0;
    bool local_38 = false;
    bool local_39 = false;
    t = (input_2.tangent.xyz - (n_1 * dot(n_1, input_2.tangent.xyz)));
    handedness = ((input_2.tangent.w >= 0.0) ? 1.0 : -1.0);
    if (!((abs(input_2.tangent.w) < 0.5))) {
        vec3 _e26 = t;
        vec3 _e27 = t;
        local_38 = (dot(_e26, _e27) < 1e-8);
    } else {
        local_38 = true;
    }
    bool _e32 = local_38;
    if (_e32) {
        vec3 dp1_ = dFdx(input_2.worldPosition);
        vec3 dp2_ = dFdy(input_2.worldPosition);
        vec2 duv1_ = dFdx(input_2.uv);
        vec2 duv2_ = dFdy(input_2.uv);
        float determinant_2 = ((duv1_.x * duv2_.y) - (duv1_.y * duv2_.x));
        float signUV = ((determinant_2 >= 0.0) ? 1.0 : -1.0);
        vec3 raw = (((dp1_ * duv2_.y) - (dp2_ * duv1_.y)) * signUV);
        t = (raw - (n_1 * dot(n_1, raw)));
        vec3 b_4 = (((dp2_ * duv1_.x) - (dp1_ * duv2_.x)) * signUV);
        vec3 _e68 = t;
        handedness = ((dot(cross(n_1, _e68), b_4) >= 0.0) ? 1.0 : -1.0);
        vec3 _e76 = t;
        vec3 _e77 = t;
        if (!((dot(_e76, _e77) < 1e-8))) {
            local_39 = (abs(determinant_2) < 1e-8);
        } else {
            local_39 = true;
        }
        bool _e88 = local_39;
        if (_e88) {
            vec3 axis_2 = ((abs(n_1.y) > 0.99) ? vec3(1.0, 0.0, 0.0) : vec3(0.0, 1.0, 0.0));
            t = cross(axis_2, n_1);
            handedness = 1.0;
        }
    }
    vec3 _e104 = t;
    t = normalize(_e104);
    vec3 _e106 = t;
    vec3 _e107 = t;
    float _e109 = handedness;
    return mat3x3(_e106, (cross(n_1, _e107) * _e109), n_1);
}

vec3 pbr_mapped_normal(mat3x3 frame_1, vec3 mapped, float scale) {
    vec3 value = (frame_1 * vec3((mapped.xy * scale), mapped.z));
    return ((dot(value, value) > 1e-8) ? (value / vec3(max(length(value), 1e-6))) : frame_1[2]);
}

float light_range_window(float distanceSquared, float range) {
    float ratio = (distanceSquared / max((range * range), 1e-8));
    float window = max((1.0 - (ratio * ratio)), 0.0);
    return (window * window);
}

float light_distance_attenuation(float distanceSquared_1, float range_1, float radius) {
    float _e3 = light_range_window(distanceSquared_1, range_1);
    return (_e3 / max(distanceSquared_1, max((radius * radius), 0.0001)));
}

vec3 area_lighting(uint index_3, vec3 position_8, PBRSurface s_3, vec3 v_6) {
    vec3 result_1 = vec3(0.0);
    uint sampleIndex_1 = 0u;
    vec3 offset = vec3(0.0);
    float measure = 0.0;
    float weight_1 = 0.0;
    float kind = _group_0_binding_0_fs.lightDirections[index_3].w;
    vec4 _e12 = _group_0_binding_0_fs.lightPosition[index_3];
    vec3 center_2 = _e12.xyz;
    vec4 right = _group_0_binding_0_fs.lightRight[index_3];
    vec4 up = _group_0_binding_0_fs.lightUp[index_3];
    vec4 _e25 = _group_0_binding_0_fs.lightDirections[index_3];
    vec3 forward = _e25.xyz;
    bool loop_init_7 = true;
    while(true) {
        if (!loop_init_7) {
            uint _e152 = sampleIndex_1;
            sampleIndex_1 = (_e152 + 1u);
        }
        loop_init_7 = false;
        uint _e32 = sampleIndex_1;
        if ((_e32 < 16u)) {
        } else {
            break;
        }
        {
            offset = vec3(0.0);
            measure = ((right.w * up.w) / 16.0);
            if ((kind == 6.0)) {
                uint _e46 = sampleIndex_1;
                uint _e50 = sampleIndex_1;
                vec2 uv_8 = (((vec2(float((_e46 % 4u)), float((_e50 / 4u))) + vec2(0.5)) / vec2(4.0)) - vec2(0.5));
                offset = ((right.xyz * (uv_8.x * right.w)) + (up.xyz * (uv_8.y * up.w)));
            } else {
                if ((kind == 7.0)) {
                    uint _e77 = sampleIndex_1;
                    float radius_2 = (sqrt(((float(_e77) + 0.5) / 16.0)) * right.w);
                    uint _e86 = sampleIndex_1;
                    float angle_1 = (float(_e86) * 2.3999631);
                    offset = (radius_2 * ((cos(angle_1) * right.xyz) + (sin(angle_1) * up.xyz)));
                    measure = (((PI * right.w) * right.w) / 16.0);
                } else {
                    uint _e106 = sampleIndex_1;
                    offset = (right.xyz * ((((float(_e106) + 0.5) / 16.0) - 0.5) * right.w));
                    measure = (right.w / 16.0);
                }
            }
            vec3 _e120 = offset;
            vec3 delta_3 = ((center_2 + _e120) - position_8);
            float distanceSquared_2 = dot(delta_3, delta_3);
            vec3 l_2 = (delta_3 * inversesqrt(max(distanceSquared_2, 1e-8)));
            float _e128 = measure;
            float _e133 = _group_0_binding_0_fs.lightParams[index_3].x;
            float _e135 = light_distance_attenuation(distanceSquared_2, _e133, 0.0);
            weight_1 = (_e128 * _e135);
            if ((kind != 8.0)) {
                float _e140 = weight_1;
                weight_1 = (_e140 * max(dot(forward, -(l_2)), 0.0));
            }
            vec3 _e146 = result_1;
            vec3 _e147 = pbr_brdf(s_3, v_6, l_2);
            float _e148 = weight_1;
            result_1 = (_e146 + (_e147 * _e148));
        }
    }
    vec3 _e154 = result_1;
    vec4 _e158 = _group_0_binding_0_fs.lightColors[index_3];
    float _e165 = _group_0_binding_0_fs.lightParams[index_3].y;
    return ((_e154 * _e158.xyz) * _e165);
}

float grid_coverage(vec2 point, float step_, float pixels) {
    vec2 uv_9 = (point / vec2(step_));
    vec2 _e5 = fwidth(uv_9);
    vec2 footprint = max(_e5, vec2(1e-6));
    vec2 distance_4 = (abs((fract((uv_9 - vec2(0.5))) - vec2(0.5))) / footprint);
    vec2 lines = (vec2(1.0) - smoothstep(vec2(((pixels * 0.5) - 0.5)), vec2(((pixels * 0.5) + 0.5)), distance_4));
    vec2 visibility_2 = (vec2(1.0) - smoothstep(vec2(0.25), vec2(0.75), footprint));
    return max((lines.x * visibility_2.x), (lines.y * visibility_2.y));
}

vec4 editor_grid(PBRInput input_3) {
    float step_1 = input_3.custom0_.x;
    float blend_1 = input_3.custom0_.y;
    float pixels_2 = input_3.custom0_.z;
    uint axis_3 = uint(input_3.custom0_.w);
    vec2 point_2 = vec2(input_3.worldPosition[((axis_3 + 1u) % 3u)], input_3.worldPosition[((axis_3 + 2u) % 3u)]);
    float _e23 = grid_coverage(point_2, step_1, pixels_2);
    float _e26 = grid_coverage(point_2, (step_1 * 10.0), pixels_2);
    float _e29 = grid_coverage(point_2, (step_1 * 100.0), pixels_2);
    float coverage = mix(max((_e23 * 0.24), (_e26 * 0.48)), max((_e26 * 0.24), (_e29 * 0.48)), blend_1);
    float distance_5 = length((point_2 - input_3.custom1_.xy));
    float fade_1 = (1.0 - smoothstep((input_3.custom1_.z * 0.4), input_3.custom1_.z, distance_5));
    vec4 _e56 = _group_0_binding_0_fs.cameraPosition;
    vec3 delta_4 = (_e56.xyz - input_3.worldPosition);
    vec3 direction_6 = ((input_3.custom2_.w > 0.5) ? input_3.custom2_.xyz : (delta_4 / vec3(max(length(delta_4), 1e-6))));
    float angle_2 = smoothstep(0.015, 0.15, abs(direction_6[axis_3]));
    float alpha_3 = (((coverage * fade_1) * angle_2) * input_3.baseColor.w);
    return vec4((input_3.baseColor.xyz * alpha_3), alpha_3);
}

vec4 fxaa_sample(vec2 uv_4, vec2 pixel) {
    vec4 _e13 = textureLod(tex, vec2(clamp(uv_4, (pixel * 0.5), (vec2(1.0) - (pixel * 0.5)))), 0.0);
    return _e13;
}

float fxaa_luma(vec4 color) {
    return (dot(color.xyz, vec3(0.299, 0.587, 0.114)) + (color.w * 0.25));
}

vec4 fxaa(vec2 uvIn) {
    vec2 direction_4 = vec2(0.0);
    bool local_40 = false;
    vec2 uv_10 = vec2(uvIn.x, (SHADOW_FLIP_Y ? (1.0 - uvIn.y) : uvIn.y));
    vec2 pixel_5 = (vec2(1.0) / vec2(uvec2(textureSize(tex, 0).xy)));
    vec4 _e16 = fxaa_sample(uv_10, pixel_5);
    vec4 _e22 = fxaa_sample((uv_10 + (vec2(-1.0, -1.0) * pixel_5)), pixel_5);
    float _e23 = fxaa_luma(_e22);
    vec4 _e29 = fxaa_sample((uv_10 + (vec2(1.0, -1.0) * pixel_5)), pixel_5);
    float _e30 = fxaa_luma(_e29);
    vec4 _e36 = fxaa_sample((uv_10 + (vec2(-1.0, 1.0) * pixel_5)), pixel_5);
    float _e37 = fxaa_luma(_e36);
    vec4 _e39 = fxaa_sample((uv_10 + pixel_5), pixel_5);
    float _e40 = fxaa_luma(_e39);
    float _e41 = fxaa_luma(_e16);
    float low = min(_e41, min(min(_e23, _e30), min(_e37, _e40)));
    float high = max(_e41, max(max(_e23, _e30), max(_e37, _e40)));
    if (((high - low) < max(0.03125, (high * 0.125)))) {
        return _e16;
    }
    direction_4 = vec2(-((((_e23 + _e30) - _e37) - _e40)), (((_e23 + _e37) - _e30) - _e40));
    float reduction = max(((((_e23 + _e30) + _e37) + _e40) * 0.03125), 0.0078125);
    vec2 _e72 = direction_4;
    float _e74 = direction_4.x;
    float _e77 = direction_4.y;
    direction_4 = (clamp((_e72 / vec2((min(abs(_e74), abs(_e77)) + reduction))), vec2(-8.0), vec2(8.0)) * pixel_5);
    vec2 _e89 = direction_4;
    vec4 _e94 = fxaa_sample((uv_10 - (_e89 / vec2(6.0))), pixel_5);
    vec2 _e95 = direction_4;
    vec4 _e100 = fxaa_sample((uv_10 + (_e95 / vec2(6.0))), pixel_5);
    vec4 narrow = ((_e94 + _e100) * 0.5);
    vec2 _e106 = direction_4;
    vec4 _e110 = fxaa_sample((uv_10 - (_e106 * 0.5)), pixel_5);
    vec2 _e111 = direction_4;
    vec4 _e115 = fxaa_sample((uv_10 + (_e111 * 0.5)), pixel_5);
    vec4 wide = ((narrow * 0.5) + ((_e110 + _e115) * 0.25));
    float _e120 = fxaa_luma(wide);
    if (!((_e120 < low))) {
        local_40 = (_e120 > high);
    } else {
        local_40 = true;
    }
    bool _e127 = local_40;
    return (_e127 ? narrow : wide);
}

void main() {
    PBRInput input_ = PBRInput(gl_FragCoord, gl_FrontFacing, _vs2fs_location0, _vs2fs_location1, _vs2fs_location2, _vs2fs_location3, _vs2fs_location4, _vs2fs_location5, _vs2fs_location6, _vs2fs_location7, _vs2fs_location8, _vs2fs_location9, _vs2fs_location10);
    vec4 base = vec4(0.0);
    bool local = false;
    bool local_1 = false;
    bool local_2 = false;
    bool local_3 = false;
    bool local_4 = false;
    float metallic = 0.0;
    float roughness = 0.0;
    vec3 n = vec3(0.0);
    bool local_5 = false;
    float ao = 1.0;
    bool local_6 = false;
    bool local_7 = false;
    mat3x3 frame = mat3x3(0.0);
    bool local_8 = false;
    bool local_9 = false;
    bool local_10 = false;
    vec3 coatNormal = vec3(0.0);
    float coat = 0.0;
    float coatRoughness = 0.0;
    vec3 sheen = vec3(0.0);
    float sheenRoughness = 0.0;
    float anisotropy = 0.0;
    vec2 direction = vec2(1.0, 0.0);
    bool local_11 = false;
    vec3 tangent = vec3(0.0);
    vec3 bitangent = vec3(0.0);
    vec2 rotation = vec2(0.0);
    vec3 projectedTangent = vec3(0.0);
    float specular = 0.0;
    vec3 specColor = vec3(0.0);
    vec3 emission = vec3(0.0);
    vec2 sheenResponse = vec2(0.0);
    vec3 lit = vec3(0.0);
    vec3 indirect = vec3(0.0);
    float shadow = 1.0;
    uint index = 0u;
    vec3 l = vec3(0.0);
    float attenuation = 0.0;
    bool local_12 = false;
    bool local_13 = false;
    bool local_14 = false;
    float angle = 0.0;
    bool local_15 = false;
    bool local_16 = false;
    float _e4 = _group_0_binding_0_fs.renderParams.y;
    if ((_e4 == 7.0)) {
        vec4 _e8 = fxaa(input_.uv);
        _fs2p_location0 = _e8;
        return;
    }
    vec4 texel_1 = texture(tex, vec2(input_.uv));
    if ((input_.material.x < -1.5)) {
        float alpha_4 = (texel_1.w * input_.baseColor.w);
        vec3 tint = ((input_.custom0_.w > 0.5) ? ((((vec3(texel_1.w) - texel_1.xyz) * input_.custom0_.xyz) + (texel_1.xyz * input_.baseColor.xyz)) * input_.baseColor.w) : ((((vec3(1.0) - texel_1.xyz) * input_.custom0_.xyz) + (texel_1.xyz * input_.baseColor.xyz)) * alpha_4));
        _fs2p_location0 = vec4(tint, alpha_4);
        return;
    }
    uint flags_1 = uint((input_.material.w + 0.5));
    if (((flags_1 & 8192u) != 0u)) {
        vec4 _e64 = editor_grid(input_);
        _fs2p_location0 = _e64;
        return;
    }
    base = (texel_1 * input_.baseColor);
    if (((flags_1 & 3u) == 1u)) {
        float _e75 = base.w;
        local = (_e75 < input_.material.z);
    } else {
        local = false;
    }
    bool _e80 = local;
    if (_e80) {
        discard;
    }
    float _e85 = _group_0_binding_0_fs.renderParams.w;
    bool front = ((input_.front != (_e85 > 0.5)) != ((flags_1 & 1024u) != 0u));
    if ((input_.material.x >= 0.0)) {
        if (((flags_1 & 512u) == 0u)) {
            local_1 = !(front);
        } else {
            local_1 = false;
        }
        bool _e106 = local_1;
        if (_e106) {
            discard;
        }
        if (((flags_1 & 3u) != 2u)) {
            base.w = 1.0;
        }
    }
    float _e116 = _group_0_binding_0_fs.renderParams.y;
    if (!((_e116 == 1.0))) {
        float _e125 = _group_0_binding_0_fs.renderParams.y;
        local_2 = (_e125 == 2.0);
    } else {
        local_2 = true;
    }
    bool _e129 = local_2;
    if (!(_e129)) {
        float _e136 = _group_0_binding_0_fs.renderParams.y;
        local_3 = (_e136 == 5.0);
    } else {
        local_3 = true;
    }
    bool _e140 = local_3;
    if (_e140) {
        uint depth_3 = uint((clamp(input_.position.z, 0.0, 1.0) * 16777215.0));
        _fs2p_location0 = vec4((vec3(float(((depth_3 >> 16u) & 255u)), float(((depth_3 >> 8u) & 255u)), float((depth_3 & 255u))) / vec3(255.0)), 1.0);
        return;
    }
    float _e171 = _group_0_binding_0_fs.renderParams.y;
    if ((_e171 == 3.0)) {
        local_4 = (input_.material.y < 0.0);
    } else {
        local_4 = false;
    }
    bool _e181 = local_4;
    if (_e181) {
        _fs2p_location0 = vec4(0.0);
        return;
    }
    if (((flags_1 & 2048u) != 0u)) {
        vec4 _e191 = _group_0_binding_0_fs.cameraPosition;
        vec3 direction_7 = ((input_.custom3_.w > 0.5) ? input_.custom3_.xyz : normalize((input_.worldPosition - _e191.xyz)));
        if (((flags_1 & 16384u) != 0u)) {
            vec3 pole = ((direction_7.y < 0.0) ? input_.custom2_.xyz : input_.custom0_.xyz);
            vec4 _e221 = pbr_output(mix(input_.custom1_.xyz, pole, sqrt(abs(direction_7.y))), 1.0);
            _fs2p_location0 = _e221;
            return;
        }
        if (((flags_1 & 8388608u) != 0u)) {
            vec3 _e226 = pbr_panorama(direction_7);
            vec4 _e228 = pbr_output(_e226, 1.0);
            _fs2p_location0 = _e228;
            return;
        }
        vec3 _e230 = pbr_environment(direction_7, 0.0);
        vec4 _e232 = pbr_output(_e230, 1.0);
        _fs2p_location0 = _e232;
        return;
    }
    if ((input_.material.x < 0.0)) {
        vec4 _e237 = base;
        float _e240 = base.w;
        float _e243 = base.w;
        _fs2p_location0 = vec4((_e237.xyz * _e240), _e243);
        return;
    }
    if (((flags_1 & 256u) != 0u)) {
        float _e252 = _group_0_binding_0_fs.renderParams.y;
        if ((_e252 == 3.0)) {
            vec4 _e255 = base;
            vec4 _e257 = pbr_encode_radiance(_e255.xyz);
            _fs2p_location0 = _e257;
            return;
        }
        float _e261 = _group_0_binding_0_fs.renderParams.y;
        if ((_e261 == 6.0)) {
            _fs2p_location0 = vec4(1.0);
            return;
        }
        vec4 _e266 = base;
        vec3 _e268 = pbr_fog_color();
        vec3 _e269 = pbr_tone_map(_e268);
        float _e271 = pbr_fog_factor(input_.worldPosition);
        vec3 color_1 = mix(_e266.xyz, _e269, _e271);
        vec3 _e273 = pbr_srgb(color_1);
        float _e275 = base.w;
        float _e278 = base.w;
        _fs2p_location0 = vec4((_e273 * _e275), _e278);
        return;
    }
    metallic = input_.emissiveMetallic.w;
    roughness = input_.material.x;
    if (((flags_1 & 8u) != 0u)) {
        vec4 mr = texture(metallicRoughnessTex, vec2(input_.uv));
        float _e294 = metallic;
        metallic = (_e294 * mr.z);
        float _e297 = roughness;
        roughness = clamp((_e297 * mr.y), 0.045, 1.0);
    }
    vec3 _e304 = dFdx(input_.normal);
    vec3 _e307 = dFdy(input_.normal);
    vec3 normalVariation = max(abs(_e304), abs(_e307));
    float geometryRoughness = max(max(normalVariation.x, normalVariation.y), normalVariation.z);
    float _e315 = roughness;
    roughness = min(1.0, (max(_e315, 0.0525) + geometryRoughness));
    n = normalize(input_.normal);
    if (((flags_1 & 512u) != 0u)) {
        local_5 = !(front);
    } else {
        local_5 = false;
    }
    bool _e332 = local_5;
    if (_e332) {
        vec3 _e333 = n;
        n = -(_e333);
    }
    vec3 shadowNormal = n;
    if (((flags_1 & 16u) != 0u)) {
        vec4 _e345 = texture(occlusionTex, vec2(input_.uv));
        ao = mix(1.0, _e345.x, input_.custom0_.y);
    }
    float _e354 = _group_0_binding_0_fs.renderParams.y;
    if ((_e354 != 4.0)) {
        local_6 = ((flags_1 & 32768u) == 0u);
    } else {
        local_6 = false;
    }
    bool _e364 = local_6;
    if (_e364) {
        local_7 = ((flags_1 & 3u) != 2u);
    } else {
        local_7 = false;
    }
    bool _e372 = local_7;
    if (_e372) {
        float _e373 = ao;
        float _e375 = pbr_screen_occlusion(input_.worldPosition, shadowNormal);
        ao = (_e373 * _e375);
    }
    float _e380 = _group_0_binding_0_fs.renderParams.y;
    if ((_e380 == 6.0)) {
        float _e383 = ao;
        _fs2p_location0 = vec4(_e383, 0.0, 0.0, 1.0);
        return;
    }
    vec3 _e396 = n;
    frame = mat3x3(vec3(1.0, 0.0, 0.0), vec3(0.0, 1.0, 0.0), _e396);
    if (!(((flags_1 & 4u) != 0u))) {
        if ((input_.custom0_.w > 0.0)) {
            local_9 = ((flags_1 & 262144u) != 0u);
        } else {
            local_9 = false;
        }
        bool _e417 = local_9;
        local_8 = _e417;
    } else {
        local_8 = true;
    }
    bool _e419 = local_8;
    if (!(_e419)) {
        local_10 = (input_.custom3_.x > 0.0);
    } else {
        local_10 = true;
    }
    bool _e428 = local_10;
    if (_e428) {
        vec3 _e429 = n;
        mat3x3 _e430 = pbr_basis(input_, _e429);
        frame = _e430;
    }
    vec3 _e431 = n;
    coatNormal = _e431;
    if (((flags_1 & 4u) != 0u)) {
        mat3x3 _e437 = frame;
        vec4 _e441 = texture(normalTex, vec2(input_.uv));
        vec3 _e450 = pbr_mapped_normal(_e437, ((_e441.xyz * 2.0) - vec3(1.0)), input_.custom0_.x);
        n = _e450;
    }
    coat = clamp(input_.custom0_.w, 0.0, 1.0);
    coatRoughness = input_.custom1_.w;
    float _e460 = coat;
    if ((_e460 > 0.0)) {
        if (((flags_1 & 65536u) != 0u)) {
            float _e467 = coat;
            vec4 _e471 = texture(clearcoatTex, vec2(input_.uv));
            coat = (_e467 * _e471.x);
        }
        if (((flags_1 & 131072u) != 0u)) {
            float _e478 = coatRoughness;
            vec4 _e482 = texture(clearcoatRoughnessTex, vec2(input_.uv));
            coatRoughness = (_e478 * _e482.y);
        }
        if (((flags_1 & 262144u) != 0u)) {
            mat3x3 _e489 = frame;
            vec4 _e493 = texture(clearcoatNormalTex, vec2(input_.uv));
            vec3 _e502 = pbr_mapped_normal(_e489, ((_e493.xyz * 2.0) - vec3(1.0)), input_.custom3_.w);
            coatNormal = _e502;
        }
    }
    float _e503 = coatRoughness;
    coatRoughness = min(1.0, (max(_e503, 0.0525) + geometryRoughness));
    sheen = clamp(input_.custom2_.xyz, vec3(0.0), vec3(1.0));
    sheenRoughness = input_.custom2_.w;
    vec3 _e520 = sheen;
    if (any(greaterThan(_e520, vec3(0.0)))) {
        if (((flags_1 & 524288u) != 0u)) {
            vec3 _e529 = sheen;
            vec4 _e533 = texture(sheenColorTex, vec2(input_.uv));
            sheen = (_e529 * _e533.xyz);
        }
        if (((flags_1 & 1048576u) != 0u)) {
            float _e540 = sheenRoughness;
            vec4 _e544 = texture(sheenRoughnessTex, vec2(input_.uv));
            sheenRoughness = (_e540 * _e544.w);
        }
    }
    anisotropy = clamp(input_.custom3_.x, 0.0, 1.0);
    float _e557 = anisotropy;
    if ((_e557 > 0.0)) {
        local_11 = ((flags_1 & 2097152u) != 0u);
    } else {
        local_11 = false;
    }
    bool _e567 = local_11;
    if (_e567) {
        vec4 sample_ = texture(anisotropyTex, vec2(input_.uv));
        vec2 encoded_1 = ((sample_.xy * 2.0) - vec2(1.0));
        if ((dot(encoded_1, encoded_1) > 1e-8)) {
            direction = normalize(encoded_1);
        }
        float _e582 = anisotropy;
        anisotropy = (_e582 * sample_.z);
    }
    vec3 _e586 = frame[0];
    tangent = _e586;
    vec3 _e589 = frame[1];
    bitangent = _e589;
    float _e591 = anisotropy;
    if ((_e591 > 0.0)) {
        rotation = input_.custom3_.yz;
        vec2 _e600 = rotation;
        vec2 _e601 = rotation;
        vec2 _e602 = rotation;
        rotation = ((dot(_e601, _e602) > 1e-8) ? _e600 : vec2(1.0, 0.0));
        float _e608 = rotation.x;
        float _e610 = direction.x;
        float _e613 = rotation.y;
        float _e615 = direction.y;
        float _e619 = rotation.y;
        float _e621 = direction.x;
        float _e624 = rotation.x;
        float _e626 = direction.y;
        direction = vec2(((_e608 * _e610) - (_e613 * _e615)), ((_e619 * _e621) + (_e624 * _e626)));
        vec3 _e631 = frame[0];
        float _e633 = direction.x;
        vec3 _e636 = frame[1];
        float _e638 = direction.y;
        vec3 rawTangent = ((_e631 * _e633) + (_e636 * _e638));
        vec3 _e641 = n;
        vec3 _e642 = n;
        projectedTangent = (rawTangent - (_e641 * dot(_e642, rawTangent)));
        vec3 _e647 = projectedTangent;
        vec3 _e648 = projectedTangent;
        if ((dot(_e647, _e648) < 1e-8)) {
            vec3 _e653 = frame[1];
            vec3 _e654 = n;
            projectedTangent = cross(_e653, _e654);
        }
        vec3 _e656 = projectedTangent;
        tangent = normalize(_e656);
        vec3 _e658 = n;
        vec3 _e659 = tangent;
        bitangent = cross(_e658, _e659);
    }
    vec4 _e663 = _group_0_binding_0_fs.cameraPosition;
    vec4 _e670 = _group_0_binding_0_fs.cameraForward;
    float _e676 = _group_0_binding_0_fs.cameraPosition.w;
    vec3 v_7 = ((_e676 < 0.0) ? -(_e670.xyz) : normalize((_e663.xyz - input_.worldPosition)));
    specular = input_.custom0_.z;
    specColor = input_.custom1_.xyz;
    if (((flags_1 & 64u) != 0u)) {
        float _e690 = specular;
        vec4 _e694 = texture(specularTex, vec2(input_.uv));
        specular = (_e690 * _e694.w);
    }
    if (((flags_1 & 128u) != 0u)) {
        vec3 _e701 = specColor;
        vec4 _e705 = texture(specularColorTex, vec2(input_.uv));
        specColor = (_e701 * _e705.xyz);
    }
    float iorRatio = ((input_.material.y == 0.0) ? 1.0 : (1.0 - (2.0 / (max(input_.material.y, 1.0) + 1.0))));
    vec3 _e726 = specColor;
    float _e731 = specular;
    vec3 dielectricF0_ = (min((vec3((iorRatio * iorRatio)) * _e726), vec3(1.0)) * _e731);
    float diffuseF0_ = max(max(dielectricF0_.x, dielectricF0_.y), dielectricF0_.z);
    vec4 _e738 = base;
    float _e740 = metallic;
    vec3 f0_2 = mix(dielectricF0_, _e738.xyz, _e740);
    float _e742 = specular;
    float _e743 = metallic;
    float f90_2 = mix(_e742, 1.0, _e743);
    emission = input_.emissiveMetallic.xyz;
    if (((flags_1 & 32u) != 0u)) {
        vec3 _e753 = emission;
        vec4 _e757 = texture(emissiveTex, vec2(input_.uv));
        emission = (_e753 * _e757.xyz);
    }
    vec3 _e760 = n;
    float nv_4 = max(dot(_e760, v_7), 0.0001);
    float _e767 = sheenRoughness;
    sheenRoughness = clamp(_e767, 0.045, 1.0);
    vec3 _e771 = sheen;
    if (any(greaterThan(_e771, vec3(0.0)))) {
        float _e776 = sheenRoughness;
        vec2 _e777 = pbr_sheen_response(nv_4, _e776);
        sheenResponse = _e777;
    }
    float sheenAlbedo = sheenResponse.y;
    float _e781 = sheen.x;
    float _e783 = sheen.y;
    float _e786 = sheen.z;
    float sheenScale = (1.0 - (max(max(_e781, _e783), _e786) * sheenAlbedo));
    float _e791 = coat;
    vec3 _e792 = coatNormal;
    float coatFresnel = (_e791 * (0.04 + (0.96 * pow((1.0 - clamp(abs(dot(_e792, v_7)), 0.0, 1.0)), 5.0))));
    vec3 _e807 = n;
    vec3 _e808 = coatNormal;
    vec3 _e809 = tangent;
    vec3 _e810 = bitangent;
    vec4 _e811 = base;
    float _e813 = metallic;
    float _e814 = roughness;
    float _e815 = specular;
    float _e816 = coat;
    float _e817 = coatRoughness;
    vec3 _e821 = sheen;
    float _e822 = sheenRoughness;
    float _e824 = sheenResponse.x;
    float _e828 = anisotropy;
    PBRSurface surface = PBRSurface(_e807, _e808, _e809, _e810, _e811.xyz, _e813, _e814, f0_2, f90_2, diffuseF0_, _e815, _e816, clamp(_e817, 0.045, 1.0), coatFresnel, _e821, _e822, sheenAlbedo, sheenScale, min((_e824 * nv_4), 1.0), _e828);
    float _e833 = _group_0_binding_0_fs.renderParams.y;
    if ((_e833 == 4.0)) {
        float _e836 = ao;
        vec4 _e837 = pbr_resolve_reflections(input_, surface, v_7, _e836, flags_1);
        _fs2p_location0 = _e837;
        return;
    }
    float _e838 = specular;
    float ambientF = (diffuseF0_ + ((_e838 - diffuseF0_) * pow((1.0 - clamp(nv_4, 0.0, 1.0)), 5.0)));
    float _e851 = metallic;
    float diffuseWeight = ((1.0 - ambientF) * (1.0 - _e851));
    vec4 _e855 = base;
    vec4 _e859 = _group_0_binding_0_fs.ambientLight;
    float _e864 = ao;
    vec3 _e866 = emission;
    lit = ((((((_e855.xyz * _e859.xyz) * diffuseWeight) * sheenScale) * _e864) + _e866) * (1.0 - coatFresnel));
    float _e875 = _group_0_binding_0_fs.environment.x;
    if ((_e875 > 0.0)) {
        vec3 _e878 = n;
        vec3 _e880 = pbr_environment(_e878, 6.0);
        vec3 _e881 = pbr_reflection_normal(surface, v_7);
        float _e882 = roughness;
        vec3 _e883 = pbr_environment_reflection(_e881, v_7, _e882);
        float _e884 = roughness;
        vec4 _e887 = pbr_atlas(vec2(nv_4, _e884), 7.0);
        vec2 brdf_1 = _e887.xy;
        vec3 dielectricSingle = ((dielectricF0_ * brdf_1.x) + vec3((f90_2 * brdf_1.y)));
        vec4 _e895 = base;
        vec3 metallicSingle = ((_e895.xyz * brdf_1.x) + vec3((f90_2 * brdf_1.y)));
        vec3 _e903 = pbr_multiscattering(dielectricF0_, f90_2, brdf_1);
        vec4 _e904 = base;
        vec3 _e906 = pbr_multiscattering(_e904.xyz, f90_2, brdf_1);
        float _e907 = metallic;
        vec3 single_1 = mix(dielectricSingle, metallicSingle, _e907);
        float _e909 = metallic;
        vec3 multiple = mix(_e903, _e906, _e909);
        vec4 _e911 = base;
        float _e913 = metallic;
        vec3 diffuse_1 = ((_e911.xyz * (1.0 - _e913)) * max(vec3(0.0), ((vec3(1.0) - dielectricSingle) - _e903)));
        indirect = (((_e883 * single_1) + (_e880 * (multiple + diffuse_1))) * sheenScale);
        vec3 _e931 = sheen;
        if (any(greaterThan(_e931, vec3(0.0)))) {
            vec3 _e936 = indirect;
            vec3 _e937 = n;
            float _e938 = sheenRoughness;
            vec3 _e939 = pbr_environment_reflection(_e937, v_7, _e938);
            vec3 _e940 = sheen;
            indirect = (_e936 + ((_e939 * _e940) * sheenAlbedo));
        }
        vec3 _e944 = indirect;
        indirect = (_e944 * (1.0 - coatFresnel));
        float _e948 = coat;
        if ((_e948 > 0.0)) {
            vec3 _e951 = coatNormal;
            float cnv_1 = max(dot(_e951, v_7), 0.0001);
            vec4 _e958 = pbr_atlas(vec2(cnv_1, surface.coatRoughness), 7.0);
            vec2 coatBRDF = _e958.xy;
            vec3 _e960 = indirect;
            vec3 _e961 = coatNormal;
            vec3 _e963 = pbr_environment_reflection(_e961, v_7, surface.coatRoughness);
            float _e964 = coat;
            indirect = (_e960 + ((_e963 * _e964) * ((0.04 * coatBRDF.x) + coatBRDF.y)));
        }
        vec3 _e973 = lit;
        vec3 _e974 = indirect;
        float _e978 = _group_0_binding_0_fs.environment.x;
        float _e980 = ao;
        lit = (_e973 + ((_e974 * _e978) * _e980));
    }
    float _e986 = _group_0_binding_0_fs.lightMeta.x;
    if ((_e986 == 0.0)) {
        if (((flags_1 & 4096u) == 0u)) {
            float _e997 = pbr_light_shadow(0u, input_.worldPosition, shadowNormal);
            shadow = _e997;
        }
        vec3 _e998 = lit;
        vec4 _e1001 = _group_0_binding_0_fs.lightDirection;
        vec3 _e1005 = pbr_brdf(surface, v_7, normalize(-(_e1001.xyz)));
        vec4 _e1008 = _group_0_binding_0_fs.directionalLight;
        float _e1011 = shadow;
        lit = (_e998 + ((_e1005 * _e1008.xyz) * _e1011));
    }
    bool loop_init_8 = true;
    while(true) {
        if (!loop_init_8) {
            uint _e1240 = index;
            index = (_e1240 + 1u);
        }
        loop_init_8 = false;
        uint _e1016 = index;
        if ((_e1016 < 16u)) {
        } else {
            break;
        }
        {
            uint _e1019 = index;
            float _e1024 = _group_0_binding_0_fs.lightMeta.x;
            if ((float(_e1019) >= _e1024)) {
                break;
            }
            uint _e1028 = index;
            float kind_1 = _group_0_binding_0_fs.lightDirections[_e1028].w;
            if ((kind_1 == 0.0)) {
                continue;
            }
            if ((kind_1 >= 6.0)) {
                vec3 _e1036 = lit;
                uint _e1037 = index;
                vec3 _e1039 = area_lighting(_e1037, input_.worldPosition, surface, v_7);
                lit = (_e1036 + _e1039);
                continue;
            }
            uint _e1043 = index;
            vec4 _e1045 = _group_0_binding_0_fs.lightDirections[_e1043];
            l = -(_e1045.xyz);
            attenuation = 1.0;
            if ((kind_1 >= 2.0)) {
                uint _e1055 = index;
                vec4 _e1057 = _group_0_binding_0_fs.lightPosition[_e1055];
                vec3 delta_5 = (_e1057.xyz - input_.worldPosition);
                float distanceSquared_3 = dot(delta_5, delta_5);
                uint _e1064 = index;
                float range_2 = _group_0_binding_0_fs.lightParams[_e1064].x;
                uint _e1070 = index;
                vec4 _e1072 = _group_0_binding_0_fs.lightDirections[_e1070];
                vec3 forward_1 = _e1072.xyz;
                uint _e1076 = index;
                vec4 right_1 = _group_0_binding_0_fs.lightRight[_e1076];
                uint _e1081 = index;
                vec4 up_1 = _group_0_binding_0_fs.lightUp[_e1081];
                if ((kind_1 == 5.0)) {
                    float depth_4 = dot(-(delta_5), forward_1);
                    if (!((depth_4 < 0.0))) {
                        local_12 = (depth_4 >= range_2);
                    } else {
                        local_12 = true;
                    }
                    bool _e1095 = local_12;
                    if (!(_e1095)) {
                        local_13 = (abs(dot(delta_5, right_1.xyz)) > right_1.w);
                    } else {
                        local_13 = true;
                    }
                    bool _e1105 = local_13;
                    if (!(_e1105)) {
                        local_14 = (abs(dot(delta_5, up_1.xyz)) > up_1.w);
                    } else {
                        local_14 = true;
                    }
                    bool _e1115 = local_14;
                    if (_e1115) {
                        continue;
                    }
                    l = -(forward_1);
                    uint _e1119 = index;
                    float _e1122 = _group_0_binding_0_fs.lightParams[_e1119].y;
                    attenuation = _e1122;
                    uint _e1125 = index;
                    float _e1128 = _group_0_binding_0_fs.lightParams[_e1125].z;
                    if ((_e1128 > 0.5)) {
                        float _e1131 = attenuation;
                        float _e1133 = light_range_window((depth_4 * depth_4), range_2);
                        attenuation = (_e1131 * _e1133);
                    }
                } else {
                    if ((distanceSquared_3 >= (range_2 * range_2))) {
                        continue;
                    }
                    l = (delta_5 * inversesqrt(max(distanceSquared_3, 1e-8)));
                    uint _e1143 = index;
                    float _e1146 = _group_0_binding_0_fs.lightColors[_e1143].w;
                    float _e1147 = light_distance_attenuation(distanceSquared_3, range_2, _e1146);
                    uint _e1150 = index;
                    float _e1153 = _group_0_binding_0_fs.lightParams[_e1150].y;
                    attenuation = (_e1147 * _e1153);
                    if ((kind_1 == 3.0)) {
                        vec3 _e1157 = l;
                        float cone = dot(forward_1, -(_e1157));
                        uint _e1162 = index;
                        float inner = _group_0_binding_0_fs.lightParams[_e1162].z;
                        uint _e1168 = index;
                        float outer = _group_0_binding_0_fs.lightParams[_e1168].w;
                        angle = ((cone >= outer) ? 1.0 : 0.0);
                        if ((inner > outer)) {
                            angle = clamp(((cone - outer) / (inner - outer)), 0.0, 1.0);
                        }
                        float _e1184 = attenuation;
                        float _e1185 = angle;
                        float _e1186 = angle;
                        attenuation = (_e1184 * (_e1185 * _e1186));
                    } else {
                        if ((kind_1 == 4.0)) {
                            float depth_5 = dot(-(delta_5), forward_1);
                            if (!((depth_5 <= 0.0))) {
                                local_15 = (abs(dot(delta_5, right_1.xyz)) > (depth_5 * right_1.w));
                            } else {
                                local_15 = true;
                            }
                            bool _e1205 = local_15;
                            if (!(_e1205)) {
                                local_16 = (abs(dot(delta_5, up_1.xyz)) > (depth_5 * up_1.w));
                            } else {
                                local_16 = true;
                            }
                            bool _e1216 = local_16;
                            if (_e1216) {
                                continue;
                            }
                        }
                    }
                }
            }
            if (((flags_1 & 4096u) == 0u)) {
                float _e1221 = attenuation;
                uint _e1222 = index;
                float _e1224 = pbr_light_shadow(_e1222, input_.worldPosition, shadowNormal);
                attenuation = (_e1221 * _e1224);
            }
            vec3 _e1226 = lit;
            vec3 _e1227 = l;
            vec3 _e1228 = pbr_brdf(surface, v_7, _e1227);
            uint _e1231 = index;
            vec4 _e1233 = _group_0_binding_0_fs.lightColors[_e1231];
            float _e1236 = attenuation;
            lit = (_e1226 + ((_e1228 * _e1233.xyz) * _e1236));
        }
    }
    vec3 _e1242 = lit;
    float _e1244 = base.w;
    vec4 _e1246 = pbr_surface_output(_e1242, _e1244, input_.worldPosition);
    _fs2p_location0 = _e1246;
    return;
}
