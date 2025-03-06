import React from 'react';

function ServiceStatus({ service }) {
  return (
    <div>
      <h2>{service.Name}</h2>
      <p>Status: {service.Status}</p>
      {service.Status === 'up' && (
        <button onClick={() => window.location.href = `/service/${service.Name}`}>
          Access {service.Name}
        </button>
      )}
    </div>
  );
}

export default ServiceStatus;