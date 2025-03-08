import React, { useState, useEffect } from 'react';
import ServiceStatus from './components/ServiceStatus';
import './App.css';

function App() {
  const [services, setServices] = useState([]);
  const [error, setError] = useState(null);

  useEffect(() => {
    const fetchServices = async () => {
      try {
        const response = await fetch('/api/services');
        if (!response.ok) {
          throw new Error(`HTTP error! status: ${response.status}`);
        }
        const data = await response.json();
        setServices(data);
        setError(null);
      } catch (error) {
        console.error("Could not fetch services:", error);
        setError(error.message);
      }
    };

    fetchServices();
    const intervalId = setInterval(fetchServices, 5000); // 5秒ごとに更新

    return () => clearInterval(intervalId); // クリーンアップ関数
  }, []);

  return (
    <div>
      <h1>Service Status</h1>
      {services.map((service) => (
        <ServiceStatus key={service.Name} service={service} />
      ))}
    </div>
  );
}

export default App;