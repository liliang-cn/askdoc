// AskDoc SDK Entry Point

export { AskDocWidget } from './widget';
export { APIClient } from './api';
export type {
  AskDocConfig,
  WidgetConfig,
  ChatRequest,
  ChatResponse,
  Source,
  StreamChunk,
} from './types';

// Auto-initialize
import { AskDocWidget } from './widget';

if (typeof window !== 'undefined') {
  // Make available globally
  (window as any).AskDoc = {
    init: (config: any) => {
      const widget = new AskDocWidget(config);
      widget.init();
      return widget;
    },
  };
}
