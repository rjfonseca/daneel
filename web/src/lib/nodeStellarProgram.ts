/**
 * @file nodeStellarProgram.ts
 * Programa WebGL customizado para renderização de estrelas galácticas no Sigma.js.
 *
 * Em vez de discos de confete opacos e planos, renderiza estrelas como esferas
 * incandescentes de gás em fusão nuclear com:
 * 1. Núcleo incandescente (branco-quente de fusão termonuclear no centro r < 0.40).
 * 2. Manto plasmático na cor espectral da estrela (médio raio).
 * 3. Queda suave de opacidade radial (halo coronal / corona gasosa suave r > 0.45).
 * 4. Suporte a PICKING_MODE nativo do Sigma para garantir que o mouse picking
 *    (hover, tooltips, click) funcione com 100% de fidelidade.
 */

import { NodeCircleProgram } from 'sigma/rendering';

// language=GLSL
const STELLAR_FRAGMENT_SHADER = /*glsl*/ `
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
    // Raio normalizado de 0.0 (centro) a 1.0 (borda da coroa)
    float r = dist / v_radius;

    // v_radius representa o raio físico da estrela na tela em pixels.
    // Em vez de uma coroa percentual fixa (que cria um borrão gigante de 16px+ no zoom profundo),
    // delimitamos a coroa gasosa entre 1.5px e 3.5px no máximo, preservando alta definição.
    float blurPixels = clamp(v_radius * 0.18, 1.5, 3.5);
    float innerR = clamp(1.0 - (blurPixels / v_radius), 0.50, 0.92);

    // 1. Halo coronal nítido e contido
    float edgeAlpha = smoothstep(1.0, innerR, r);
    float alpha = edgeAlpha * v_color.a;

    // 2. Núcleo incandescente de fusão com alta definição e foco
    float coreRadius = clamp(0.35, 0.15, innerR * 0.65);
    float coreIgnition = smoothstep(coreRadius, 0.0, r);
    vec3 stellarColor = mix(v_color.rgb, vec3(1.0, 1.0, 1.0), coreIgnition * 0.92);

    // 3. Manto plasmático suave na cor espectral
    float mantleGlow = smoothstep(innerR, 0.10, r) * 0.18;
    stellarColor += v_color.rgb * mantleGlow;

    // Sigma utiliza premultiplied alpha (gl.blendFunc(gl.ONE, gl.ONE_MINUS_SRC_ALPHA))
    gl_FragColor = vec4(stellarColor * alpha, alpha);
  }
  #endif
}
`;

export class NodeStellarProgram extends NodeCircleProgram {
  getDefinition() {
    const def = super.getDefinition();
    return {
      ...def,
      FRAGMENT_SHADER_SOURCE: STELLAR_FRAGMENT_SHADER,
    };
  }
}
