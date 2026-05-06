/**
 * Calcula a idade de uma data em relação ao momento atual.
 * Retorna uma string curta (agora, 5m, 2h, 12d, 3m, 2a).
 */
export const formatAge = (timestamp: string | number | Date | null | undefined): string => {
  if (!timestamp) return '';
  try {
    const date = new Date(timestamp);
    if (Number.isNaN(date.getTime())) return '';

    const now = new Date();
    const diffInSeconds = Math.floor((now.getTime() - date.getTime()) / 1000);

    const labels: Record<string, string> = {
      now: 'agora',
      minute: 'm',
      hour: 'h',
      day: 'd',
      month: 'mês',
      year: 'a',
    };

    if (diffInSeconds < 60) return labels.now;
    const diffInMinutes = Math.floor(diffInSeconds / 60);
    if (diffInMinutes < 60) return `${diffInMinutes}${labels.minute}`;
    const diffInHours = Math.floor(diffInMinutes / 60);
    if (diffInHours < 24) return `${diffInHours}${labels.hour}`;
    const diffInDays = Math.floor(diffInHours / 24);
    if (diffInDays < 30) return `${diffInDays}${labels.day}`;
    const diffInMonths = Math.floor(diffInDays / 30);
    if (diffInMonths < 12) return `${diffInMonths}${labels.month}`;
    const diffInYears = Math.floor(diffInMonths / 12);
    return `${diffInYears}${labels.year}`;
  } catch (_e) {
    return '';
  }
};
