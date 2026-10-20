import type Sigma from 'sigma';
import type Graph from 'graphology';
import type { GalaxyNode, GalaxyLink, SigmaContextState } from '../../src/types/galaxy';

interface RenderedNode extends Partial<GalaxyNode> {
  x: number;
  y: number;
  hidden?: boolean;
}

type BrowserSigma = Omit<Pick<Sigma, keyof Sigma>, 'getGraph' | 'emit'> & {
  getGraph(): Graph<RenderedNode, Partial<GalaxyLink>>;
  normalizationFunction?: (point: { x: number; y: number }) => { x: number; y: number };
  emit(event: 'clickNode', payload: { node: string }): boolean;
};

declare global {
  interface Window {
    __sigmaInstance: BrowserSigma | null;
    __sigmaContext: SigmaContextState | undefined;
  }
}
