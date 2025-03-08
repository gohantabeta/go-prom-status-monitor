import React from 'react';
import './ServiceStatus.css';


function ServiceStatus({ service }) {
  const cardClassName = `service-card ${service.status === 'up' ? 'active' : 'inactive'}`;

  return (
    <div className={cardClassName}>
      <h2 className="service-name">{service.name}</h2>
      <div className="service-status">
        <span className={`status-indicator ${service.status}`}>
          {service.status === 'up' ? 'Active' : 'Inactive'}
        </span>
      </div>
      {service.status === 'up' && (
        <button 
          className="access-button"
          onClick={() => window.location.href = `/service/${service.name}`}
        >
          Access Service
        </button>
      )}
    </div>
  );
}

export default ServiceStatus;