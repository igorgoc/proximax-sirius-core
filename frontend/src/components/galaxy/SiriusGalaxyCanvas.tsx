import React, { useRef, useEffect, useState, useCallback } from 'react';
import {
  GalaxyStarData,
  ConstellationEdge,
  BackgroundParticle,
  generateBackgroundParticles,
  formatCompactXPX,
} from '../../utils/galaxyMath';

interface ActiveNovaState {
  x: number;
  y: number;
  publicKey?: string;
  text?: string;
  startTime: number;
}

export interface SiriusGalaxyCanvasProps {
  stars: GalaxyStarData[];
  edges: ConstellationEdge[];
  selectedStar: GalaxyStarData | null;
  onSelectStar: (star: GalaxyStarData | null) => void;
  activeNova?: { x: number; y: number; publicKey?: string; text?: string } | null;
  showFilaments: boolean;
  showLabels: boolean;
  onCameraResetRef?: React.MutableRefObject<(() => void) | null>;
  onCenterStarRef?: React.MutableRefObject<((star: GalaxyStarData) => void) | null>;
}

export const SiriusGalaxyCanvas: React.FC<SiriusGalaxyCanvasProps> = ({
  stars,
  edges,
  selectedStar,
  onSelectStar,
  activeNova,
  showFilaments,
  showLabels,
  onCameraResetRef,
  onCenterStarRef,
}) => {
  const canvasRef = useRef<HTMLCanvasElement | null>(null);
  const containerRef = useRef<HTMLDivElement | null>(null);

  // Camera state: world origin at center of viewport
  const cameraRef = useRef({
    x: 0,
    y: 0,
    zoom: 1.0,
    targetX: 0,
    targetY: 0,
    targetZoom: 1.0,
    isDragging: false,
    dragStartX: 0,
    dragStartY: 0,
    camStartX: 0,
    camStartY: 0,
  });

  const [hoveredStar, setHoveredStar] = useState<GalaxyStarData | null>(null);
  const particlesRef = useRef<BackgroundParticle[]>([]);
  const novaQueueRef = useRef<ActiveNovaState[]>([]);
  const lastNovaPropRef = useRef<{ x: number; y: number; publicKey?: string; text?: string } | null>(null);

  // Synchronized refs to allow steady 60 FPS render loop without effect teardown
  const starsRef = useRef(stars);
  starsRef.current = stars;
  const edgesRef = useRef(edges);
  edgesRef.current = edges;
  const selectedStarRef = useRef(selectedStar);
  selectedStarRef.current = selectedStar;
  const hoveredStarRef = useRef(hoveredStar);
  hoveredStarRef.current = hoveredStar;
  const showFilamentsRef = useRef(showFilaments);
  showFilamentsRef.current = showFilaments;
  const showLabelsRef = useRef(showLabels);
  showLabelsRef.current = showLabels;
  const hasInitiallyCenteredRef = useRef(false);

  // Track active Nova shockwaves
  useEffect(() => {
    if (activeNova && activeNova !== lastNovaPropRef.current) {
      lastNovaPropRef.current = activeNova;
      novaQueueRef.current.push({
        x: activeNova.x,
        y: activeNova.y,
        publicKey: activeNova.publicKey,
        text: activeNova.text,
        startTime: performance.now(),
      });
      // Limit to 4 concurrent shockwaves
      if (novaQueueRef.current.length > 4) {
        novaQueueRef.current.shift();
      }
    }
  }, [activeNova]);

  // Initialize background stardust once
  useEffect(() => {
    particlesRef.current = generateBackgroundParticles(160, 1400);
  }, []);

  // Camera control handlers for parent HUD: Frame-to-Fit all stars in viewport
  const resetCamera = useCallback(() => {
    const currentStars = starsRef.current;
    const cam = cameraRef.current;
    if (currentStars.length === 0) {
      cam.targetX = 0;
      cam.targetY = 0;
      cam.targetZoom = 1.0;
      return;
    }

    const container = containerRef.current;
    const canvas = canvasRef.current;
    const dpr = window.devicePixelRatio || 1;
    const viewportWidth = container?.clientWidth || (canvas ? canvas.width / dpr : 800);
    const viewportHeight = container?.clientHeight || (canvas ? canvas.height / dpr : 600);

    // Compute celestial bounding box of all stars
    let minX = Infinity;
    let maxX = -Infinity;
    let minY = Infinity;
    let maxY = -Infinity;

    for (let i = 0; i < currentStars.length; i++) {
      const s = currentStars[i];
      if (s.x < minX) minX = s.x;
      if (s.x > maxX) maxX = s.x;
      if (s.y < minY) minY = s.y;
      if (s.y > maxY) maxY = s.y;
    }

    // Safety padding for glow halos, orbit rings, and star text labels
    const padding = 120;
    const spanX = Math.max(120, (maxX - minX) + padding * 2);
    const spanY = Math.max(120, (maxY - minY) + padding * 2);

    const centerX = (minX + maxX) * 0.5;
    const centerY = (minY + maxY) * 0.5;

    // Ideal zoom to frame the whole constellation comfortably
    const zoomX = viewportWidth / spanX;
    const zoomY = viewportHeight / spanY;
    const fitZoom = Math.min(zoomX, zoomY);

    // Clamp zoom to comfortable viewing range
    const targetZoom = Math.max(0.25, Math.min(1.5, fitZoom));

    cam.targetX = -centerX;
    cam.targetY = -centerY;
    cam.targetZoom = targetZoom;
  }, []);

  const centerOnStar = useCallback((star: GalaxyStarData) => {
    cameraRef.current.targetX = -star.x;
    cameraRef.current.targetY = -star.y;
    cameraRef.current.targetZoom = 1.45;
  }, []);

  useEffect(() => {
    if (onCameraResetRef) onCameraResetRef.current = resetCamera;
    if (onCenterStarRef) onCenterStarRef.current = centerOnStar;
  }, [resetCamera, centerOnStar, onCameraResetRef, onCenterStarRef]);

  // Frame all stars initially on first load
  useEffect(() => {
    if (hasInitiallyCenteredRef.current) return;
    if (stars.length > 0) {
      hasInitiallyCenteredRef.current = true;
      resetCamera();
    }
  }, [stars, resetCamera]);

  // Main 60 FPS Render Loop
  useEffect(() => {
    const canvas = canvasRef.current;
    if (!canvas) return;
    const ctx = canvas.getContext('2d');
    if (!ctx) return;

    let animId: number;

    const render = (time: number) => {
      const cam = cameraRef.current;

      // Smooth camera interpolation (ease-out lerp)
      cam.x += (cam.targetX - cam.x) * 0.12;
      cam.y += (cam.targetY - cam.y) * 0.12;
      cam.zoom += (cam.targetZoom - cam.zoom) * 0.12;

      const dpr = window.devicePixelRatio || 1;
      const width = canvas.width / dpr;
      const height = canvas.height / dpr;

      if (width <= 0 || height <= 0) {
        animId = requestAnimationFrame(render);
        return;
      }

      ctx.save();
      ctx.scale(dpr, dpr);

      // 1. Deep Space Cosmic Void Background
      const bgGrad = ctx.createRadialGradient(
        width * 0.5,
        height * 0.5,
        50,
        width * 0.5,
        height * 0.5,
        Math.max(width, height) * 0.8
      );
      bgGrad.addColorStop(0, '#0E1420'); // Galactic core deep blue
      bgGrad.addColorStop(0.5, '#080B11'); // Dark void
      bgGrad.addColorStop(1, '#040608'); // Outer cosmos
      ctx.fillStyle = bgGrad;
      ctx.fillRect(0, 0, width, height);

      // World coordinate space transformation
      const centerX = width * 0.5;
      const centerY = height * 0.5;

      const worldToScreen = (wx: number, wy: number) => {
        return {
          x: centerX + (wx + cam.x) * cam.zoom,
          y: centerY + (wy + cam.y) * cam.zoom,
        };
      };

      // 2. Render Twinkling Background Particles
      const particles = particlesRef.current;
      for (let i = 0; i < particles.length; i++) {
        const p = particles[i];
        const screenPos = worldToScreen(p.x, p.y);

        // Cull out of bounds
        if (
          screenPos.x < -20 ||
          screenPos.x > width + 20 ||
          screenPos.y < -20 ||
          screenPos.y > height + 20
        ) {
          continue;
        }

        const twinkle = Math.sin(time * p.twinkleSpeed + p.phase);
        const alpha = Math.max(0.1, p.baseAlpha + twinkle * 0.25);

        ctx.fillStyle = p.color;
        ctx.globalAlpha = alpha;
        ctx.beginPath();
        ctx.arc(screenPos.x, screenPos.y, p.size * Math.min(1.5, cam.zoom), 0, Math.PI * 2);
        ctx.fill();
      }
      ctx.globalAlpha = 1.0;

      // 3. Render Constellation Filaments (Mesh lines)
      if (showFilamentsRef.current) {
        const currentEdges = edgesRef.current;
        for (let i = 0; i < currentEdges.length; i++) {
          const edge = currentEdges[i];
          const p1 = worldToScreen(edge.x1, edge.y1);
          const p2 = worldToScreen(edge.x2, edge.y2);

          ctx.beginPath();
          ctx.moveTo(p1.x, p1.y);
          ctx.lineTo(p2.x, p2.y);
          ctx.strokeStyle = '#38BDF8';
          ctx.globalAlpha = edge.opacity * Math.min(1, cam.zoom * 0.9);
          ctx.lineWidth = Math.max(0.75, 1.2 * cam.zoom);
          ctx.stroke();
        }
        ctx.globalAlpha = 1.0;
      }

      // 4. Render Active Harvest Nova Shockwaves
      const now = performance.now();
      novaQueueRef.current = novaQueueRef.current.filter((nova) => {
        const age = (now - nova.startTime) / 1000;
        const duration = 2.4; // 2.4s lifetime
        if (age >= duration) return false;

        const progress = age / duration;
        const screenPos = worldToScreen(nova.x, nova.y);
        const waveRadius = (20 + progress * 220) * cam.zoom;
        const alpha = Math.max(0, (1 - progress) * 0.85);

        // Expanding Photon Ring
        ctx.save();
        ctx.beginPath();
        ctx.arc(screenPos.x, screenPos.y, waveRadius, 0, Math.PI * 2);
        ctx.strokeStyle = `rgba(245, 158, 11, ${alpha})`;
        ctx.lineWidth = Math.max(1.5, 3.5 * (1 - progress) * cam.zoom);
        ctx.shadowColor = '#F59E0B';
        ctx.shadowBlur = 15;
        ctx.stroke();

        // Optional +XPX Badge
        if (nova.text && progress < 0.8) {
          const badgeY = screenPos.y - waveRadius * 0.5 - 15;
          ctx.font = 'bold 11px monospace';
          ctx.fillStyle = `rgba(252, 211, 77, ${alpha * 1.1})`;
          ctx.textAlign = 'center';
          ctx.fillText(nova.text, screenPos.x, badgeY);
        }
        ctx.restore();

        return true;
      });

      // 5. Render Validator Stars
      const currentStars = starsRef.current;
      const currentHovered = hoveredStarRef.current;
      const currentSelected = selectedStarRef.current;
      const currentShowLabels = showLabelsRef.current;

      // Map active harvest glow intensities for winning block harvesters
      const nowTime = performance.now();
      const harvestPulseMap = new Map<string, number>();
      for (let m = 0; m < novaQueueRef.current.length; m++) {
        const nova = novaQueueRef.current[m];
        if (nova.publicKey) {
          const age = (nowTime - nova.startTime) / 1000;
          const duration = 2.4;
          if (age < duration) {
            const intensity = Math.max(0, 1 - age / duration);
            const prev = harvestPulseMap.get(nova.publicKey) || 0;
            harvestPulseMap.set(nova.publicKey, Math.max(prev, intensity));
          }
        }
      }

      for (let i = 0; i < currentStars.length; i++) {
        const star = currentStars[i];
        const pos = worldToScreen(star.x, star.y);

        // Cull stars outside viewport margin
        if (
          pos.x < -60 ||
          pos.x > width + 60 ||
          pos.y < -60 ||
          pos.y > height + 60
        ) {
          continue;
        }

        const isHovered = currentHovered?.publicKey === star.publicKey;
        const isSelected = currentSelected?.publicKey === star.publicKey;
        const isHarvestPulsing = harvestPulseMap.get(star.publicKey) || 0;
        const scaledRadius = Math.max(4, star.radius * cam.zoom);

        // A. Pulsing Corona Glow (Outer Aura)
        const basePulse = 0.85 + 0.15 * Math.sin(time * 0.003 + i);
        const pulse = basePulse + isHarvestPulsing * 0.65;
        const glowRadius = scaledRadius * (star.haloSize / star.radius) * pulse;

        const glowGrad = ctx.createRadialGradient(
          pos.x,
          pos.y,
          scaledRadius * 0.4,
          pos.x,
          pos.y,
          glowRadius
        );
        glowGrad.addColorStop(0, isHarvestPulsing > 0.1 ? '#FDE68A' : star.color);
        glowGrad.addColorStop(0.35, isHarvestPulsing > 0.1 ? '#F59E0B' : star.glowColor);
        glowGrad.addColorStop(1, 'rgba(0, 0, 0, 0)');

        ctx.fillStyle = glowGrad;
        ctx.beginPath();
        ctx.arc(pos.x, pos.y, glowRadius, 0, Math.PI * 2);
        ctx.fill();

        // Extra Golden Photon Ring for block harvester (stays fixed at position)
        if (isHarvestPulsing > 0) {
          ctx.save();
          ctx.beginPath();
          ctx.arc(pos.x, pos.y, scaledRadius * (1.2 + (1 - isHarvestPulsing) * 2.0), 0, Math.PI * 2);
          ctx.strokeStyle = `rgba(245, 158, 11, ${isHarvestPulsing * 0.95})`;
          ctx.lineWidth = 2 + isHarvestPulsing * 3;
          ctx.shadowColor = '#F59E0B';
          ctx.shadowBlur = 20;
          ctx.stroke();
          ctx.restore();
        }

        // B. Solid Star Core
        ctx.save();
        ctx.beginPath();
        ctx.arc(pos.x, pos.y, scaledRadius, 0, Math.PI * 2);
        ctx.fillStyle = star.color;
        ctx.shadowColor = isHarvestPulsing > 0.1 ? '#F59E0B' : star.glowColor;
        ctx.shadowBlur = isSelected || isHovered ? 25 : (isHarvestPulsing > 0.1 ? 30 : 12);
        ctx.fill();
        ctx.restore();

        // C. Sirius Self Validator: Rotating Planetary Orbit Ring
        if (star.isSelf) {
          ctx.save();
          ctx.translate(pos.x, pos.y);
          ctx.rotate(time * 0.0006); // Slow majestic rotation
          ctx.setLineDash([6, 5]);
          ctx.strokeStyle = 'rgba(16, 185, 129, 0.7)';
          ctx.lineWidth = 1.6;
          ctx.beginPath();
          ctx.arc(0, 0, scaledRadius * 2.1, 0, Math.PI * 2);
          ctx.stroke();

          // Outer secondary faint ring
          ctx.setLineDash([3, 8]);
          ctx.strokeStyle = 'rgba(6, 182, 212, 0.4)';
          ctx.beginPath();
          ctx.arc(0, 0, scaledRadius * 2.8, 0, Math.PI * 2);
          ctx.stroke();
          ctx.restore();
        }

        // D. Sci-Fi Targeting Crosshair Reticle (Hovered or Selected)
        if (isHovered || isSelected) {
          const reticleRadius = scaledRadius * 2.6 + 6;
          ctx.save();
          ctx.translate(pos.x, pos.y);
          ctx.strokeStyle = isSelected ? '#10B981' : '#38BDF8';
          ctx.lineWidth = 1.4;
          ctx.shadowColor = isSelected ? '#10B981' : '#38BDF8';
          ctx.shadowBlur = 8;

          // Rotating dashed border
          ctx.save();
          ctx.rotate(-time * 0.0012);
          ctx.setLineDash([8, 6]);
          ctx.beginPath();
          ctx.arc(0, 0, reticleRadius, 0, Math.PI * 2);
          ctx.stroke();
          ctx.restore();

          // 4 Corner brackets
          const bracketLen = 7;
          const b = reticleRadius * 0.9;
          ctx.beginPath();
          // Top-left
          ctx.moveTo(-b - bracketLen, -b);
          ctx.lineTo(-b, -b);
          ctx.lineTo(-b, -b - bracketLen);
          // Top-right
          ctx.moveTo(b + bracketLen, -b);
          ctx.lineTo(b, -b);
          ctx.lineTo(b, -b - bracketLen);
          // Bottom-left
          ctx.moveTo(-b - bracketLen, b);
          ctx.lineTo(-b, b);
          ctx.lineTo(-b, b + bracketLen);
          // Bottom-right
          ctx.moveTo(b + bracketLen, b);
          ctx.lineTo(b, b);
          ctx.lineTo(b, b + bracketLen);
          ctx.stroke();

          ctx.restore();
        }

        // E. Star Label: Only the compact amount of staked XPX (e.g. 7.7M)
        if (currentShowLabels || isHovered || isSelected || star.isSelf) {
          const label = formatCompactXPX(star.stakedBalanceXPX);

          ctx.save();
          ctx.font = star.isSelf ? 'bold 11px monospace' : '10px monospace';
          ctx.fillStyle = star.isSelf
            ? '#A7F3D0'
            : isSelected
            ? '#BAE6FD'
            : isHarvestPulsing > 0.1
            ? '#FDE68A'
            : 'rgba(226, 232, 240, 0.9)';
          ctx.textAlign = 'center';
          ctx.shadowColor = 'rgba(0,0,0,0.9)';
          ctx.shadowBlur = 4;
          ctx.fillText(label, pos.x, pos.y + scaledRadius + 14);
          ctx.restore();
        }
      }

      ctx.restore();
      animId = requestAnimationFrame(render);
    };

    animId = requestAnimationFrame(render);
    return () => cancelAnimationFrame(animId);
  }, []);

  // Canvas Resize Observer with DPR Scaling (No inline style px to prevent flex ratchet loops)
  useEffect(() => {
    const container = containerRef.current;
    const canvas = canvasRef.current;
    if (!container || !canvas) return;

    const handleResize = () => {
      const width = container.clientWidth;
      const height = container.clientHeight;
      if (width <= 0 || height <= 0) return;

      const dpr = window.devicePixelRatio || 1;
      const targetW = Math.round(width * dpr);
      const targetH = Math.round(height * dpr);

      if (canvas.width !== targetW || canvas.height !== targetH) {
        canvas.width = targetW;
        canvas.height = targetH;
      }
    };

    handleResize();
    const ro = new ResizeObserver(handleResize);
    ro.observe(container);
    return () => ro.disconnect();
  }, []);

  // Hit-Testing: Find star under screen coordinate
  const getStarAt = useCallback(
    (screenX: number, screenY: number): GalaxyStarData | null => {
      const canvas = canvasRef.current;
      if (!canvas) return null;
      const dpr = window.devicePixelRatio || 1;
      const width = canvas.width / dpr;
      const height = canvas.height / dpr;
      const cam = cameraRef.current;

      const centerX = width * 0.5;
      const centerY = height * 0.5;

      for (let i = 0; i < stars.length; i++) {
        const star = stars[i];
        const sx = centerX + (star.x + cam.x) * cam.zoom;
        const sy = centerY + (star.y + cam.y) * cam.zoom;
        const hitRadius = Math.max(16, star.radius * cam.zoom + 8);

        const dx = screenX - sx;
        const dy = screenY - sy;
        if (dx * dx + dy * dy <= hitRadius * hitRadius) {
          return star;
        }
      }
      return null;
    },
    [stars]
  );

  // Mouse / Touch Interaction Handlers
  const handleMouseDown = (e: React.MouseEvent<HTMLCanvasElement>) => {
    if (e.button !== 0) return; // Left click only
    const cam = cameraRef.current;
    cam.isDragging = true;
    cam.dragStartX = e.clientX;
    cam.dragStartY = e.clientY;
    cam.camStartX = cam.targetX;
    cam.camStartY = cam.targetY;
  };

  const handleMouseMove = (e: React.MouseEvent<HTMLCanvasElement>) => {
    const rect = canvasRef.current?.getBoundingClientRect();
    if (!rect) return;
    const screenX = e.clientX - rect.left;
    const screenY = e.clientY - rect.top;

    const cam = cameraRef.current;
    if (cam.isDragging) {
      const dx = (e.clientX - cam.dragStartX) / cam.zoom;
      const dy = (e.clientY - cam.dragStartY) / cam.zoom;
      cam.targetX = cam.camStartX + dx;
      cam.targetY = cam.camStartY + dy;
    } else {
      // Hover detection
      const found = getStarAt(screenX, screenY);
      setHoveredStar(found);
    }
  };

  const handleMouseUp = (e: React.MouseEvent<HTMLCanvasElement>) => {
    const cam = cameraRef.current;
    const wasDrag =
      Math.abs(e.clientX - cam.dragStartX) > 4 ||
      Math.abs(e.clientY - cam.dragStartY) > 4;

    cam.isDragging = false;

    if (!wasDrag) {
      const rect = canvasRef.current?.getBoundingClientRect();
      if (rect) {
        const screenX = e.clientX - rect.left;
        const screenY = e.clientY - rect.top;
        const clicked = getStarAt(screenX, screenY);
        onSelectStar(clicked);
      }
    }
  };

  const handleWheel = (e: React.WheelEvent<HTMLCanvasElement>) => {
    e.preventDefault();
    const cam = cameraRef.current;
    const zoomFactor = e.deltaY < 0 ? 1.15 : 0.87;
    const newZoom = Math.max(0.2, Math.min(2.8, cam.targetZoom * zoomFactor));
    cam.targetZoom = newZoom;
  };

  return (
    <div
      ref={containerRef}
      className="relative w-full h-full min-h-[460px] overflow-hidden select-none bg-[#07090E] rounded-xl border border-[#262B34]"
    >
      <canvas
        ref={canvasRef}
        onMouseDown={handleMouseDown}
        onMouseMove={handleMouseMove}
        onMouseUp={handleMouseUp}
        onWheel={handleWheel}
        className={`absolute inset-0 w-full h-full block ${
          hoveredStar ? 'cursor-pointer' : cameraRef.current.isDragging ? 'cursor-grabbing' : 'cursor-grab'
        }`}
      />

      {/* Floating Canvas Controls Overlay */}
      <div className="absolute bottom-4 left-4 z-10 flex items-center space-x-2 bg-[#12151B]/85 backdrop-blur-md px-3 py-1.5 rounded-lg border border-[#2A313C] shadow-lg text-xs font-mono text-slate-300">
        <button
          onClick={resetCamera}
          className="hover:text-white px-2 py-0.5 rounded hover:bg-slate-700/50 transition-colors"
          title="Reset Camera View"
        >
          Reset
        </button>
        <span className="text-slate-600">|</span>
        <button
          onClick={() => {
            const selfStar = stars.find((s) => s.isSelf);
            if (selfStar) centerOnStar(selfStar);
          }}
          className="hover:text-emerald-400 px-2 py-0.5 rounded hover:bg-slate-700/50 transition-colors flex items-center space-x-1"
          title="Center on My Validator"
        >
          <span className="w-2 h-2 rounded-full bg-emerald-500 inline-block animate-pulse" />
          <span>My Validator</span>
        </button>
      </div>
    </div>
  );
};
