import { Image } from '@tiptap/extension-image';

export const CustomImage = Image.extend({
  addNodeView() {
    return ({ node, getPos, editor }) => {
      const container = document.createElement('div');
      container.setAttribute('data-image-container', '');
      container.style.position = 'relative';
      container.style.display = 'block';
      container.style.marginLeft = 'auto';
      container.style.marginRight = 'auto';
      container.style.marginTop = '2rem';
      container.style.marginBottom = '2rem';
      container.style.maxWidth = 'fit-content';
      container.classList.add('group');
      
      const img = document.createElement('img');
      img.src = node.attrs.src;
      img.alt = node.attrs.alt || '';
      img.style.borderRadius = '0.75rem';
      img.style.border = '1px solid #27272a'; // zinc-800
      img.style.boxShadow = '0 10px 15px -3px rgb(0 0 0 / 0.1), 0 4px 6px -4px rgb(0 0 0 / 0.1)';
      img.style.maxHeight = '500px';
      img.style.objectFit = 'contain';
      img.style.display = 'block';
      
      const button = document.createElement('button');
      button.type = 'button';
      button.innerHTML = '<svg style="width: 1rem; height: 1rem;" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M19 7l-.867 12.142A2 2 0 0116.138 21H7.862a2 2 0 01-1.995-1.858L5 7m5 4v6m4-6v6m1-10V4a1 1 0 00-1-1h-4a1 1 0 00-1 1v3M4 7h16" /></svg>';
      
      // Inline styles for the button to avoid CSS dependencies
      button.style.position = 'absolute';
      button.style.top = '0.5rem';
      button.style.right = '0.5rem';
      button.style.backgroundColor = '#ef4444'; // red-500
      button.style.color = 'white';
      button.style.padding = '0.375rem';
      button.style.borderRadius = '0.5rem';
      button.style.opacity = '0';
      button.style.transition = 'all 0.2s';
      button.style.cursor = 'pointer';
      button.style.border = 'none';
      button.style.zIndex = '10';
      button.style.display = 'flex';
      button.style.alignItems = 'center';
      button.style.justifyContent = 'center';
      button.style.boxShadow = '0 4px 6px -1px rgb(0 0 0 / 0.1)';

      // Hover effect logic
      container.onmouseenter = () => { button.style.opacity = '1'; };
      container.onmouseleave = () => { button.style.opacity = '0'; };
      button.onmouseenter = () => { button.style.backgroundColor = '#dc2626'; };
      button.onmouseleave = () => { button.style.backgroundColor = '#ef4444'; };
      
      button.onclick = (e) => {
        e.preventDefault();
        e.stopPropagation();
        
        const src = node.attrs.src;
        try {
          const url = new URL(src, window.location.origin);
          const filename = url.searchParams.get('name');
          
          if (filename) {
             window.dispatchEvent(new CustomEvent('tiptap:delete-file', { 
               detail: { 
                 filename,
                 pos: typeof getPos === 'function' ? getPos() : null
               } 
             }));
          }
        } catch (err) {
          console.error('Erro ao processar URL da imagem:', err);
        }
      };
      
      container.appendChild(img);
      container.appendChild(button);
      
      return {
        dom: container,
      };
    };
  }
});
