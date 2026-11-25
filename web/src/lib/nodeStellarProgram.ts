/**
 * @file nodeStellarProgram.ts
 * Custom Sigma.js WebGL NodeProgram wrapping the pixel-accurate stellar SDF shader.
 */

import { NodeCircleProgram } from 'sigma/rendering';
import { STELLAR_FRAGMENT_SHADER, computeStellarPixelMetrics, type StellarPixelMetrics } from './galaxy/stellarShader';

export { STELLAR_FRAGMENT_SHADER, computeStellarPixelMetrics, type StellarPixelMetrics };

export class NodeStellarProgram extends NodeCircleProgram {
  getDefinition() {
    const def = super.getDefinition();
    return {
      ...def,
      FRAGMENT_SHADER_SOURCE: STELLAR_FRAGMENT_SHADER,
    };
  }
}
