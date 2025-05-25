import React from 'react';

interface Card {
  id: number;
  name: string;
  email: string;
  created_at: string;
  updated_at: string;
}

const CardComponent: React.FC<{ card: Card }> = ({ card }) => {
  const formatDate = (dateString: string) => {
    if (!dateString) return 'N/A';
    try {
      const options: Intl.DateTimeFormatOptions = {
        year: 'numeric', month: 'short', day: 'numeric',
        hour: '2-digit', minute: '2-digit', hour12: true
      };
      return new Date(dateString).toLocaleString(undefined, options);
    } catch (e) {
      console.error("Error formatting date:", e);
      return dateString; // return original if formatting fails
    }
  };

  return (
    <div className="bg-white shadow-lg rounded-lg p-2 mb-2 hover:bg-gray-100 w-full">
      <div className="text-sm text-gray-600">Id: {card.id}</div>
      <div className="text-lg font-semibold text-gray-800">{card.name}</div>
      <div className="text-md text-gray-700">{card.email}</div>
      <div className="text-xs text-gray-500 mt-1">
        Created: {formatDate(card.created_at)}
      </div>
      <div className="text-xs text-gray-500">
        Updated: {formatDate(card.updated_at)}
      </div>
    </div>
  );
}

export default CardComponent;
