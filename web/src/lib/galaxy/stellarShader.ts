/**
 * @file stellarShader.ts
 * Pure WebGL1 Signed Distance Field (SDF) fragment shader and pixel-space
 * metric helpers for Daneel's galactic stars.
 *
 * Sigma.js v3 NodeCircleProgram passes `v_radius` and `v_diffVector` in
 * normalized framed-graph units (NOT CSS pixels). One CSS screen pixel equals
 * `u_correctionRatio * 2.0` in that space. Converting distances to true CSS
 * pixels ensures star photospheres remain razor-sharp at every zoom level.
 */

export interface StellarPixelMetrics {
  radiusPx: number;
  coronaPx: number;
  discRadiusPx: number;
  crispFraction: number;
}

/**
 * Pure mirror of the GLSL pixel-space metrics for deterministic unit testing.
 */
export function computeStellarPixelMetrics(
  vRadius: number,
  correctionRatio: number
): StellarPixelMetrics {
  const onePx = Math.max(correctionRatio * 2.0, 1e-6);
  const radiusPx = Math.max(0, vRadius) / onePx;
  const coronaPx = Math.min(3.5, Math.max(1.5, radiusPx * 0.18));
  const discRadiusPx = Math.max(radiusPx - coronaPx, radiusPx * 0.55);
  const crispFraction = radiusPx > 0 ? discRadiusPx / radiusPx : 1;

  return {
    radiusPx,
    coronaPx,
    discRadiusPx,
    crispFraction,
  };
}

// language=GLSL
export const STELLAR_FRAGMENT_SHADER = /*glsl*/ `
precision highp float;

varying vec4 v_color;
varying vec2 v_diffVector;
varying float v_radius;

uniform float u_correctionRatio;

const vec4 transparent = vec4(0.0, 0.0, 0.0, 0.0);

void main(void) {
  float dist = length(v_diffVector);

  #ifdef PICKING_MODE
  if (dist > v_radius)
    gl_FragColor = transparent;
  else
    gl_FragColor = v_color;

  #else
  if (dist > v_radius) {
    gl_FragColor = transparent;
  } else {
    // Convert framed-graph lengths into true CSS screen pixels.
    float onePx = max(u_correctionRatio * 2.0, 1e-6);
    float radiusPx = v_radius / onePx;
    float distPx = dist / onePx;

    // Pixel-bounded coronal gas thickness [1.5px, 3.5px] prevents deep-zoom blur.
    float coronaPx = clamp(radiusPx * 0.18, 1.5, 3.5);
    float discRadiusPx = max(radiusPx - coronaPx, radiusPx * 0.55);
    float aaPx = 1.15;

    // 1. Crisp anti-aliased photosphere disc + soft quadratic outer corona
    float discAlpha = smoothstep(discRadiusPx + aaPx, max(0.0, discRadiusPx - aaPx), distPx);
    float coronaFalloff = smoothstep(radiusPx, discRadiusPx, distPx);
    float coronaAlpha = coronaFalloff * coronaFalloff * 0.34;
    float alpha = clamp(discAlpha + (1.0 - discAlpha) * coronaAlpha, 0.0, 1.0) * v_color.a;

    // 2. Zoom-adaptive resolution-independent surface fidelity (WebGL1-safe)
    float rDisc = clamp(distPx / max(discRadiusPx, 1.0), 0.0, 1.0);
    float mu = sqrt(max(0.0, 1.0 - rDisc * rDisc));
    float zoomDetail = smoothstep(7.0, 22.0, radiusPx);

    // Incandescent fusion core (tighter and sharper as screen radius grows)
    float coreRadius = mix(0.42, 0.28, zoomDetail);
    float coreIgnition = smoothstep(coreRadius, 0.0, rDisc);
    vec3 baseColor = mix(v_color.rgb, vec3(1.0, 1.0, 1.0), coreIgnition * mix(0.88, 0.72, zoomDetail));

    // Spherical limb darkening gives zoomed-in stars 3D depth instead of flat blur
    float limb = mix(1.0, 0.52 + 0.48 * pow(mu, 0.55), zoomDetail);
    vec3 stellarColor = baseColor * limb;

    // Deterministic procedural photosphere granulation emerging smoothly on deep zoom
    vec2 uv = v_diffVector / max(discRadiusPx * onePx, 1e-6);
    float g1 = sin(uv.x * 11.0 + sin(uv.y * 9.0)) * cos(uv.y * 11.0 - cos(uv.x * 8.0));
    float g2 = sin((uv.x + uv.y) * 19.0) * cos((uv.x - uv.y) * 17.0);
    float granulation = (g1 * 0.6 + g2 * 0.4) * 0.11 * zoomDetail * mu;
    stellarColor += v_color.rgb * granulation;

    // Crisp chromosphere rim just inside the photosphere edge
    float rimBand = smoothstep(discRadiusPx - 2.2, discRadiusPx - 0.5, distPx) * discAlpha;
    stellarColor += mix(v_color.rgb, vec3(1.0), 0.35) * rimBand * 0.22 * zoomDetail;

    // Preserve spectral tint in outer corona halo
    stellarColor = mix(v_color.rgb * 1.15, stellarColor, discAlpha);

    // Sigma uses premultiplied alpha (gl.blendFunc(gl.ONE, gl.ONE_MINUS_SRC_ALPHA))
    gl_FragColor = vec4(clamp(stellarColor, 0.0, 1.0) * alpha, alpha);
  }
  #endif
}
`;
