import { describe, it, expect } from 'vitest';

import { 
  calculateLabelBounds, 
  calculateNebulaCalloutGeometry,
  calculateNonCollidingNebulaCallout,
  doCalloutBoxesOverlap 
} from '../src/lib/canvasMath';


describe('Bug 2: Texto fora da caixa no zoom máximo', () => {
  it('calculateLabelBounds should correctly encapsulate text of varying lengths', () => {
    const text = "Aglomerado Arquitetura";
    const size = 10;
    const ctx = { measureText: () => ({ width: 100 }) };
    
    const bounds = calculateLabelBounds(0, 0, size, text, ctx);
    
    // The box must completely encapsulate the text
    expect(bounds.width).toBeGreaterThanOrEqual(100);
    
    // If we draw text aligned 'center', its X should be exactly the center of the box.
    // If it's aligned 'left', its X should be inside the box boundaries (e.g. x + padding).
    // Let's enforce left-aligned text with proper padding:
    expect(bounds.textX).toBeGreaterThanOrEqual(bounds.x);
    expect(bounds.textX + 100).toBeLessThanOrEqual(bounds.x + bounds.width);
  });
});

describe('calculateNebulaCalloutGeometry', () => {
  it('positions text strictly above horizontal shelf for right-hand callouts (isLeft = false)', () => {
    const x = 600;
    const y = 400;
    const textWidth = 120;
    const isLeft = false;

    const geom = calculateNebulaCalloutGeometry(x, y, textWidth, isLeft);

    // Diagonal goes up and right
    expect(geom.midX).toBe(600 + 35);
    expect(geom.midY).toBe(400 - 35);

    // Horizontal shelf extends to the right
    expect(geom.endX).toBe(geom.midX + 120 + 16);
    expect(geom.endY).toBe(geom.midY);

    // Horizontal shelf range
    expect(geom.shelfMinX).toBe(geom.midX);
    expect(geom.shelfMaxX).toBe(geom.endX);

    // Text must sit vertically above the horizontal line
    expect(geom.textAnchorY).toBeLessThan(geom.endY);
    expect(geom.textAnchorY).toBe(geom.endY - 4);
    expect(geom.textBaseline).toBe('bottom');
    expect(geom.textAlign).toBe('center');

    // Text anchor must be the center of the horizontal shelf
    expect(geom.textAnchorX).toBe((geom.midX + geom.endX) / 2);

    // Text span must be strictly contained inside the horizontal shelf with 8px margin at both ends
    expect(geom.textStartX).toBe(geom.shelfMinX + 8);
    expect(geom.textEndX).toBe(geom.shelfMaxX - 8);
    expect(geom.textStartX).toBeGreaterThanOrEqual(geom.shelfMinX);
    expect(geom.textEndX).toBeLessThanOrEqual(geom.shelfMaxX);
  });

  it('positions text strictly above horizontal shelf for left-hand callouts (isLeft = true)', () => {
    const x = 300;
    const y = 250;
    const textWidth = 80;
    const isLeft = true;

    const geom = calculateNebulaCalloutGeometry(x, y, textWidth, isLeft);

    // Diagonal goes up and left
    expect(geom.midX).toBe(300 - 35);
    expect(geom.midY).toBe(250 - 35);

    // Horizontal shelf extends to the left
    expect(geom.endX).toBe(geom.midX - (80 + 16));
    expect(geom.endY).toBe(geom.midY);

    // Shelf min and max
    expect(geom.shelfMinX).toBe(geom.endX);
    expect(geom.shelfMaxX).toBe(geom.midX);

    // Text must sit vertically above the horizontal line
    expect(geom.textAnchorY).toBeLessThan(geom.endY);
    expect(geom.textAnchorY).toBe(geom.endY - 4);
    expect(geom.textBaseline).toBe('bottom');
    expect(geom.textAlign).toBe('center');

    // Text anchor must be the center of the horizontal shelf
    expect(geom.textAnchorX).toBe((geom.midX + geom.endX) / 2);

    // Text span must be strictly contained inside the horizontal shelf with 8px margin at both ends
    expect(geom.textStartX).toBe(geom.shelfMinX + 8);
    expect(geom.textEndX).toBe(geom.shelfMaxX - 8);
    expect(geom.textStartX).toBeGreaterThanOrEqual(geom.shelfMinX);
    expect(geom.textEndX).toBeLessThanOrEqual(geom.shelfMaxX);
  });

  it('respects custom options for diagonal distance, padding, and text offset', () => {
    const geom = calculateNebulaCalloutGeometry(100, 100, 50, false, {
      diagDist: 50,
      padding: 24,
      textOffsetBottom: 6,
    });

    expect(geom.midX).toBe(150);
    expect(geom.midY).toBe(50);
    expect(geom.endX).toBe(150 + 50 + 24);
    expect(geom.textAnchorY).toBe(50 - 6);
    expect(geom.textStartX).toBe(geom.shelfMinX + 12);
    expect(geom.textEndX).toBe(geom.shelfMaxX - 12);
  });

  it('adjusts elevation to avoid collision with an existing callout box', () => {
    // Place first callout at (500, 300)
    const firstResult = calculateNonCollidingNebulaCallout(500, 300, 100, false, []);
    if (!firstResult) throw new Error('Expected space for first callout');
    expect(firstResult.geom.midY).toBe(300 - 35); // 265

    // Place second callout very close below the first at (500, 320)
    const secondResult = calculateNonCollidingNebulaCallout(500, 320, 100, false, [firstResult.box]);
    
    if (!secondResult) throw new Error('Expected space for second callout');
    // It should have bumped to higher elevation (diagDist = 95)
    expect(secondResult.geom.midY).not.toBe(320 - 35);
    expect(secondResult.geom.midY).toBe(320 - 95); // 225
    
    // The two boxes should not overlap
    expect(doCalloutBoxesOverlap(firstResult.box, secondResult.box)).toBe(false);
  });
});


